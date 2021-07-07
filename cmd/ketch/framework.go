package main

import (
	"fmt"
	"io"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"

	"github.com/spf13/cobra"
)

const frameworkHelp = `
Manage frameworks.

NOTE: "pool" has been deprecated and replaced with "framework". The functionality is the same.
`

const (
	defaultIstioIngressClassName   = "istio"
	defaultTraefikIngressClassName = "traefik"
	defaultVersion                 = "v1"
)

var (
	defaultAppQuotaLimit = -1
)

func newFrameworkCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "framework",
		Aliases: []string{"pool"},
		Short:   "Manage frameworks",
		Long:    frameworkHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(newFrameworkListCmd(cfg, out))
	cmd.AddCommand(newFrameworkAddCmd(cfg, out, addFramework))
	cmd.AddCommand(newFrameworkRemoveCmd(cfg, out))
	cmd.AddCommand(newFrameworkUpdateCmd(cfg, out))
	cmd.AddCommand(newFrameworkExportCmd(cfg))
	return cmd
}

// assignDefaultsToFramework assigns default values for namespace, version, appQuotaLimit,
// ingress type, and ingress className if they are not assigned. Useful when creating or modifying
// a framework.
func assignDefaultsToFramework(framework *ketchv1.Framework) {
	framework.ObjectMeta.Name = framework.Spec.Name
	if framework.Spec.NamespaceName == "" {
		framework.Spec.NamespaceName = fmt.Sprintf("ketch-%s", framework.Spec.Name)
	}
	if framework.Spec.Version == "" {
		framework.Spec.Version = defaultVersion
	}
	if framework.Spec.AppQuotaLimit == nil {
		framework.Spec.AppQuotaLimit = &defaultAppQuotaLimit
	}
	if len(framework.Spec.IngressController.IngressType) == 0 {
		framework.Spec.IngressController.IngressType = ketchv1.TraefikIngressControllerType
	}
	if len(framework.Spec.IngressController.ClassName) == 0 {
		if framework.Spec.IngressController.IngressType.String() == defaultIstioIngressClassName {
			framework.Spec.IngressController.ClassName = defaultIstioIngressClassName
		} else {
			framework.Spec.IngressController.ClassName = defaultTraefikIngressClassName
		}
	}
}
