package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

var (
	appInfoTemplate = `Application: {{ .App.Name }}
Pool: {{ .App.Spec.Pool }} 
{{- if .App.Spec.Description }}
Description: {{ .App.Spec.Description }}
{{- end }}
{{- if .Cnames }}
{{- range $address := .Cnames }}
Address: {{ $address }}
{{- end }}
{{- else }}
The default cname hasn't assigned yet because "{{ .App.Spec.Pool }}" pool doesn't have ingress service endpoint.
{{- end }}
{{- if .App.Spec.DockerRegistry.SecretName }}
Secret name to pull application's images: {{ .App.Spec.DockerRegistry.SecretName }}
{{- end }}
{{ if .App.Spec.Env }}
Environment variables:
{{- range .App.Spec.Env }}
{{ .Name }}={{ .Value }}
{{- end }}
{{- else }}
No environment variables.
{{- end }}
{{ if .NoProcesses }}
No processes.
{{- else }}
{{ .Table }}
{{- end }}`
)

type appInfoContext struct {
	App         ketchv1.App
	Cnames      []string
	NoProcesses bool
	Table       string
}

const appInfoHelp = `
Show information about a specific app.
`

func newAppInfoCmd(cfg config, out io.Writer) *cobra.Command {
	options := appInfoOptions{}
	cmd := &cobra.Command{
		Use:   "info APPNAME",
		Short: "Show information about a specific app.",
		Args:  cobra.ExactArgs(1),
		Long:  appInfoHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.name = args[0]
			return appInfo(cmd.Context(), cfg, options, out)
		},
	}
	return cmd
}

type appInfoOptions struct {
	name string
}

func appInfo(ctx context.Context, cfg config, options appInfoOptions, out io.Writer) error {
	app := ketchv1.App{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.name}, &app); err != nil {
		return fmt.Errorf("failed to get app: %w", err)
	}
	pool := &ketchv1.Pool{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: app.Spec.Pool}, pool); err != nil {
		return fmt.Errorf("failed to get pool: %w", err)
	}

	buf := bytes.Buffer{}
	t := template.Must(template.New("app-info").Parse(appInfoTemplate))
	table := &bytes.Buffer{}
	w := tabwriter.NewWriter(table, 0, 4, 4, ' ', 0)
	fmt.Fprintln(w, "DEPLOYMENT VERSION\tIMAGE\tPROCESS NAME\tSTATE\tCMD")
	noProcesses := true
	for _, deployment := range app.Spec.Deployments {
		for _, process := range deployment.Processes {
			noProcesses = false

			pods, err := cfg.KubernetesClient().CoreV1().Pods(app.Namespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf(`theketch.io/app-name=%s,theketch.io/app-deployment-version=%s,theketch.io/app-process=%s`, app.Name, deployment.Version, process.Name),
			})
			if err != nil {
				return err
			}
			state := appState(pods.Items)

			line := []string{
				deployment.Version.String(),
				deployment.Image,
				process.Name,
				state,
				strings.Join(process.Cmd, " "),
			}
			fmt.Fprintln(w, strings.Join(line, "\t"))
		}
	}
	w.Flush()
	infoContext := appInfoContext{
		App:         app,
		Cnames:      app.CNames(pool),
		Table:       table.String(),
		NoProcesses: noProcesses,
	}
	if err := t.Execute(&buf, infoContext); err != nil {
		return err
	}
	fmt.Fprintf(out, "%v", buf.String())
	return nil
}
