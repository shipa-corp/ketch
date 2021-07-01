package main

import (
	"context"
	"errors"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

type frameworkExportOptions struct {
	frameworkName string
	filename      string
}

const frameworkExportHelp = `Export a framework's configuration file.`

var errFileExists = errors.New("file already exists")

func newFrameworkExportCmd(cfg config) *cobra.Command {
	var options frameworkExportOptions

	cmd := &cobra.Command{
		Use:   "export FRAMEWORK",
		Args:  cobra.ExactValidArgs(1),
		Short: "Export a framework to file.",
		Long:  frameworkExportHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.frameworkName = args[0]
			return exportFramework(cmd.Context(), cfg, options)
		},
	}
	cmd.Flags().StringVarP(&options.filename, "file", "f", "framework.yaml", "filename for framework export")
	return cmd
}

func exportFramework(ctx context.Context, cfg config, options frameworkExportOptions) error {
	var framework ketchv1.Framework
	err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.frameworkName}, &framework)
	if err != nil {
		return err
	}
	framework.Spec.Name = framework.Name
	// open file, err if exist, write framework.Spec
	_, err = os.Stat(options.filename)
	if !os.IsNotExist(err) {
		return errFileExists
	}
	f, err := os.Create(options.filename)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := yaml.Marshal(framework.Spec)
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	return err
}
