package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/thediveo/enumflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/validation"
)

const frameworkAddHelp = `
Add a new framework. Users can specify a new framework by name and command-line flags or
by passing a filename such as framework.yaml containing fields like:
	name: framework1
	ingressController:
	  name: istio
	  endpoint: 10.10.10.20 # load balancer ingress ip
	  type: istio
`

type ingressType enumflag.Flag

const (
	traefik ingressType = iota
	istio
)

var ingressTypeIds = map[ingressType][]string{
	traefik: {ketchv1.TraefikIngressControllerType.String()},
	istio:   {ketchv1.IstioIngressControllerType.String()},
}

type addFrameworkFn func(ctx context.Context, cfg config, options frameworkAddOptions, out io.Writer) error

func newFrameworkAddCmd(cfg config, out io.Writer, addFramework addFrameworkFn) *cobra.Command {
	options := frameworkAddOptions{}
	cmd := &cobra.Command{
		Use:   "add [FRAMEWORK|FILENAME]",
		Args:  cobra.ExactValidArgs(1),
		Short: "Add a new framework.",
		Long:  frameworkAddHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.name = args[0]
			options.ingressClassNameSet = cmd.Flags().Changed("ingress-class-name")
			return addFramework(cmd.Context(), cfg, options, out)
		},
	}
	cmd.Flags().StringVar(&options.version, "version", defaultVersion, "Version for this framework")
	cmd.Flags().StringVar(&options.namespace, "namespace", "", "Kubernetes namespace for this framework")
	cmd.Flags().IntVar(&options.appQuotaLimit, "app-quota-limit", defaultAppQuotaLimit, "Quota limit for app when adding it to this framework")
	cmd.Flags().StringVar(&options.ingressClassName, "ingress-class-name", "", `if set, it is used as kubernetes.io/ingress.class annotations. Ketch uses "istio" class name for istio ingress controller, if class name is not specified`)
	cmd.Flags().StringVar(&options.ingressClusterIssuer, "cluster-issuer", "", "ClusterIssuer to obtain SSL certificates")
	cmd.Flags().StringVar(&options.ingressServiceEndpoint, "ingress-service-endpoint", "", "an IP address or dns name of the ingress controller's Service")
	cmd.Flags().Var(enumflag.New(&options.ingressType, "ingress-type", ingressTypeIds, enumflag.EnumCaseInsensitive), "ingress-type", "ingress controller type: traefik or istio")
	cmd.RegisterFlagCompletionFunc("ingress-type", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{defaultIstioIngressClassName, defaultTraefikIngressClassName}, cobra.ShellCompDirectiveDefault
	})
	return cmd
}

type frameworkAddOptions struct {
	version string
	name    string // name may be a framework name (e.g. myframework) or filename (e.g. framework.yaml)

	appQuotaLimit int
	namespace     string

	ingressClassNameSet    bool
	ingressClassName       string
	ingressClusterIssuer   string
	ingressServiceEndpoint string
	ingressType            ingressType
}

func addFramework(ctx context.Context, cfg config, options frameworkAddOptions, out io.Writer) error {
	var framework *ketchv1.Framework
	var err error

	switch {
	case validation.ValidateYamlFilename(options.name):
		framework, err = newFrameworkFromYaml(options)
		if err != nil {
			return err
		}
	case validation.ValidateName(options.name):
		framework = newFrameworkFromArgs(options)
	default:
		return ErrInvalidFrameworkName
	}

	if len(framework.Spec.IngressController.ClusterIssuer) > 0 {
		exists, err := clusterIssuerExist(cfg.DynamicClient(), ctx, framework.Spec.IngressController.ClusterIssuer)
		if err != nil {
			return err
		}
		if !exists {
			return ErrClusterIssuerNotFound
		}
	}

	if err := cfg.Client().Create(ctx, framework); err != nil {
		return fmt.Errorf("failed to create framework: %w", err)
	}
	fmt.Fprintln(out, "Successfully added!")
	return nil
}

// newFrameworkFromYaml imports a Framework definition from a yaml file specified in options.name.
// It asserts that the framework has a name. It assigns a ketch-prefixed namespaceName, version, appQuotaLimit,
// ingressController className, and ingressController type (defaulting to traefik) if values are not specified.
func newFrameworkFromYaml(options frameworkAddOptions) (*ketchv1.Framework, error) {
	var framework ketchv1.Framework
	b, err := os.ReadFile(options.name)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(b, &framework.Spec)
	if err != nil {
		return nil, err
	}
	if len(framework.Spec.Name) == 0 {
		return nil, errors.New("a framework name is required")
	}

	assignDefaultsToFramework(&framework)

	return &framework, nil
}

// newFrameworkFromArgs creates a Framework from options. It creates a ketch-prefixed namespace if
// one is not specified.
func newFrameworkFromArgs(options frameworkAddOptions) *ketchv1.Framework {
	namespace := fmt.Sprintf("ketch-%s", options.name)
	if len(options.namespace) > 0 {
		namespace = options.namespace
	}

	framework := &ketchv1.Framework{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: options.name,
		},
		Spec: ketchv1.FrameworkSpec{
			NamespaceName: namespace,
			AppQuotaLimit: &options.appQuotaLimit,
			IngressController: ketchv1.IngressControllerSpec{
				ClassName:       options.IngressClassName(),
				ServiceEndpoint: options.ingressServiceEndpoint,
				ClusterIssuer:   options.ingressClusterIssuer,
				IngressType:     options.ingressType.ingressControllerType(),
			},
		},
		Status: ketchv1.FrameworkStatus{},
	}
	return framework
}

func (o frameworkAddOptions) IngressClassName() string {
	if !o.ingressClassNameSet && o.ingressType.ingressControllerType() == ketchv1.IstioIngressControllerType {
		return defaultIstioIngressClassName
	}

	if !o.ingressClassNameSet && o.ingressType.ingressControllerType() == ketchv1.TraefikIngressControllerType {
		return defaultTraefikIngressClassName
	}

	return o.ingressClassName
}

func (t ingressType) ingressControllerType() ketchv1.IngressControllerType {
	switch t {
	case istio:
		return ketchv1.IstioIngressControllerType
	default:
		return ketchv1.TraefikIngressControllerType
	}
}
