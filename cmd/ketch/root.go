package main

import (
	"io"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/shipa-corp/ketch/internal/templates"
)

type config interface {
	Client() client.Client
	Storage() templates.Client
	KubernetesClient() kubernetes.Interface
}

// RootCmd represents the base command when called without any subcommands
func newRootCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ketch",
		Short:   "Manage your applications and your cloud resources",
		Long:    `For details see https://theketch.io`,
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(newAppCmd(cfg, out))
	cmd.AddCommand(newCnameCmd(cfg, out))
	cmd.AddCommand(newPoolCmd(cfg, out))
	cmd.AddCommand(newUnitCmd(cfg, out))
	cmd.AddCommand(newEnvCmd(cfg, out))
	return cmd
}
