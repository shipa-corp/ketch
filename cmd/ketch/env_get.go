package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v2"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

const envGetHelp = `
Retrieve environment variables for an application.

ketch env-get [-a/--app appname] [ENVIRONMENT_VARIABLE1] [ENVIRONMENT_VARIABLE2] ...
`

func newEnvGetCmd(cfg config, out io.Writer) *cobra.Command {
	options := envGetOptions{}
	cmd := &cobra.Command{
		Use:   "get ENV_VAR1 ENV_VAR2 ...",
		Short: "Retrieve environment variables for an application.",
		Long:  envGetHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.envs = args
			return envGet(cmd.Context(), cfg, options, out, cmd.Flags())

		},
	}
	cmd.Flags().StringVarP(&options.appName, "app", "a", "", "The name of the app.")
	cmd.MarkFlagRequired("app")
	return cmd
}

type envGetOptions struct {
	appName string
	envs    []string
}

func envGet(ctx context.Context, cfg config, options envGetOptions, out io.Writer, flags *pflag.FlagSet) error {
	app := ketchv1.App{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.appName}, &app); err != nil {
		return fmt.Errorf("failed to get the app: %w", err)
	}
	envs := app.Envs(options.envs)
	outputFlag, err := flags.GetString("output")
	if err != nil {
		outputFlag = ""
	}
	switch outputFlag {
	case "json", "JSON":
		j, err := json.MarshalIndent(envs, "", "\t")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(j))
	case "yaml", "YAML":
		y, err := yaml.Marshal(envs)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(y))
	default:
		for name, value := range envs {
			fmt.Fprintf(out, "%v=%v\n", name, value)
		}
	}
	return nil
}
