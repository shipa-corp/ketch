package chart

import (
	"log"
	"os"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	ctrl "sigs.k8s.io/controller-runtime"
)

// HelmClient performs helm install and uninstall operations for provided application helm charts.
type HelmClient struct {
	cfg       *action.Configuration
	namespace string
}

// TemplateValuer is an interface that permits types that implement it (e.g. Application, Job)
// to be parameters in the UpdateChart function.
type TemplateValuer interface {
	GetValues() interface{}
	GetTemplates() map[string]string
	GetName() string
}

// NewHelmClient returns a HelmClient instance.
func NewHelmClient(namespace string) (*HelmClient, error) {
	cfg, err := getActionConfig(namespace)
	if err != nil {
		return nil, err
	}
	return &HelmClient{cfg: cfg, namespace: namespace}, nil
}

func getActionConfig(namespace string) (*action.Configuration, error) {
	actionConfig := new(action.Configuration)

	config := ctrl.GetConfigOrDie()

	// Create the ConfigFlags struct instance with initialized values from ServiceAccount
	kubeConfig := genericclioptions.NewConfigFlags(false)
	kubeConfig.APIServer = &config.Host
	kubeConfig.BearerToken = &config.BearerToken
	kubeConfig.CAFile = &config.CAFile
	kubeConfig.Namespace = &namespace
	if err := actionConfig.Init(kubeConfig, namespace, os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return nil, err
	}
	return actionConfig, nil
}

// Option to perform additional configuration of action.Install before running a chart installation.
type InstallOption func(install *action.Install)

// UpdateChart checks if the app chart is already installed and performs "helm install" or "helm update" operation.
func (c HelmClient) UpdateChart(tv TemplateValuer, config ChartConfig, opts ...InstallOption) (*release.Release, error) {
	appName := tv.GetName()
	files, err := bufferedFiles(config, tv.GetTemplates(), tv.GetValues())
	if err != nil {
		return nil, err
	}
	chrt, err := loader.LoadFiles(files)
	if err != nil {
		return nil, err
	}
	vals, err := getValuesMap(tv.GetValues())
	if err != nil {
		return nil, err
	}
	getValuesClient := action.NewGetValues(c.cfg)
	getValuesClient.AllValues = true
	_, err = getValuesClient.Run(appName)
	if err != nil && err.Error() == "release: not found" {
		clientInstall := action.NewInstall(c.cfg)
		clientInstall.ReleaseName = appName
		clientInstall.Namespace = c.namespace
		for _, opt := range opts {
			opt(clientInstall)
		}
		return clientInstall.Run(chrt, vals)
	}
	if err != nil {
		return nil, err
	}
	updateClient := action.NewUpgrade(c.cfg)
	updateClient.Namespace = c.namespace
	return updateClient.Run(appName, chrt, vals)
}

// DeleteChart uninstalls the app's helm release. It doesn't return an error if the release is not found.
func (c HelmClient) DeleteChart(appName string) error {
	uninstall := action.NewUninstall(c.cfg)
	_, err := uninstall.Run(appName)
	if err != nil && err.Error() == "release: not found" {
		return nil
	}
	return err
}
