package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/pflag"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/utils"
)

var (
	appInfoTemplate = `Application: {{ .App.Name }}
Framework: {{ .App.Spec.Framework }}
{{- if .App.Spec.Builder }}
Builder: {{ .App.Spec.Builder }}
{{- end }}
{{- if .App.Spec.Description }}
Description: {{ .App.Spec.Description }}
{{- end }}
{{- if .Cnames }}
{{- range $address := .Cnames }}
Address: {{ $address }}
{{- end }}
{{- else }}
The default cname hasn't assigned yet because "{{ .App.Spec.Framework }}" framework doesn't have ingress service endpoint.
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
{{ else }}
{{ .Table }}
{{- end }}`
)

type appInfoContext struct {
	App         ketchv1.App
	Cnames      []string
	NoProcesses bool
	Table       string
}

type appInfoOutput struct {
	AppInfoContext appInfoContext
	Deployments    []deploymentOutput
}

type deploymentOutput struct {
	DeploymentVersion string `json:"deploymentVersion" yaml:"deploymentVersion"`
	Image             string `json:"image" yaml:"image"`
	ProcessName       string `json:"processName" yaml:"processName"`
	Weight            string `json:"weight" yaml:"weight"`
	State             string `json:"state" yaml:"state"`
	Cmd               string `json:"cmd" yaml:"cmd"`
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
			return appInfo(cmd.Context(), cfg, options, out, cmd.Flags())
		},
	}
	return cmd
}

type appInfoOptions struct {
	name string
}

func appInfo(ctx context.Context, cfg config, options appInfoOptions, out io.Writer, flags *pflag.FlagSet) error {
	app := ketchv1.App{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.name}, &app); err != nil {
		return fmt.Errorf("failed to get app: %w", err)
	}
	framework := &ketchv1.Framework{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: app.Spec.Framework}, framework); err != nil {
		return fmt.Errorf("failed to get framework: %w", err)
	}

	appPods, err := cfg.KubernetesClient().CoreV1().Pods(app.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf(`%s=%s`, utils.KetchAppNameLabel, app.Name),
	})
	if err != nil {
		return err
	}

	output := generateAppInfoOutput(app, appPods, framework)
	outputFlag, err := flags.GetString("output")
	if err != nil {
		outputFlag = ""
	}
	switch outputFlag {
	case "json", "JSON":
		j, err := json.MarshalIndent(output, "", "\t")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(j))
	case "yaml", "YAML":
		y, err := yaml.Marshal(output)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(y))
	default:
		buf := bytes.Buffer{}
		t := template.Must(template.New("app-info").Parse(appInfoTemplate))
		table := &bytes.Buffer{}
		w := tabwriter.NewWriter(table, 0, 4, 4, ' ', 0)
		fmt.Fprintln(w, "DEPLOYMENT VERSION\tIMAGE\tPROCESS NAME\tWEIGHT\tSTATE\tCMD")
		for _, deployment := range output.Deployments {
			line := []string{
				deployment.DeploymentVersion, deployment.Image, deployment.ProcessName, deployment.Weight, deployment.State, deployment.Cmd,
			}
			fmt.Fprintln(w, strings.Join(line, "\t"))
		}
		w.Flush()
		output.AppInfoContext.Table = table.String()
		if err := t.Execute(&buf, output.AppInfoContext); err != nil {
			return err
		}
		fmt.Fprintf(out, "%v", buf.String())
		return nil
	}
	return nil
}

func generateAppInfoOutput(app ketchv1.App, appPods *v1.PodList, framework *ketchv1.Framework) appInfoOutput {
	noProcesses := true
	var deployments []deploymentOutput
	for _, deployment := range app.Spec.Deployments {
		for _, process := range deployment.Processes {
			noProcesses = false
			state := appState(filterProcessDeploymentPods(appPods.Items, deployment.Version.String(), process.Name))
			deployments = append(deployments, deploymentOutput{
				DeploymentVersion: deployment.Version.String(),
				Image:             deployment.Image,
				ProcessName:       process.Name,
				Weight:            fmt.Sprintf("%v%%", deployment.RoutingSettings.Weight),
				State:             state,
				Cmd:               strings.Join(process.Cmd, " "),
			})
		}
	}
	infoContext := appInfoContext{
		App:         app,
		Cnames:      app.CNames(framework),
		NoProcesses: noProcesses,
	}

	return appInfoOutput{
		infoContext, deployments,
	}
}

func filterProcessDeploymentPods(appPods []corev1.Pod, version, process string) []corev1.Pod {
	var pods []corev1.Pod
	for _, pod := range appPods {
		deploymentVersion := pod.Labels[utils.KetchDeploymentVersionLabel]
		processName := pod.Labels[utils.KetchProcessNameLabel]
		if deploymentVersion == version && processName == process {
			pods = append(pods, pod)
		}
	}
	return pods
}
