package main

import (
	"context"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/shipa-corp/ketch/cmd/ketch/configuration"
	"github.com/shipa-corp/ketch/internal/pack"
	"github.com/shipa-corp/ketch/internal/templates"
)

type config interface {
	Client() client.Client
	Storage() templates.Client
	// KubernetesClient returns kubernetes typed client. It's used to work with standard kubernetes types.
	KubernetesClient() kubernetes.Interface
	// DynamicClient returns kubernetes dynamic client. It's used to work with CRDs for which we don't have go types like ClusterIssuer.
	DynamicClient() dynamic.Interface
}

type resourceCreator interface {
	Create(context.Context, runtime.Object, ...client.CreateOption) error
}

type resourceLister interface {
	List(context.Context, runtime.Object, ...client.ListOption) error
}

type resourceGetter interface {
	Get(ctx context.Context, name types.NamespacedName, object runtime.Object) error
}

type resourceDeleter interface {
	Delete(context.Context, runtime.Object, ...client.DeleteOption) error
}

type resourceGetDeleter interface {
	resourceGetter
	resourceDeleter
}

// RootCmd represents the base command when called without any subcommands
func newRootCmd(cfg config, out io.Writer, packSvc *pack.Client, ketchConfig configuration.KetchConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ketch",
		Short:   "Manage your applications and your cloud resources",
		Long:    `For details see https://theketch.io`,
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(newAppCmd(cfg, out, packSvc, ketchConfig.DefaultBuilder))
	cmd.AddCommand(newBuilderCmd(ketchConfig, out))
	cmd.AddCommand(newCnameCmd(cfg, out))
	cmd.AddCommand(newFrameworkCmd(cfg, out))
	cmd.AddCommand(newEnvCmd(cfg, out))
	cmd.AddCommand(newCompletionCmd())
	return cmd
}
