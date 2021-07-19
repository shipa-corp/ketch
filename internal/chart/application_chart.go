// Package chart contains types and methods to convert the App CRD to an internal representation of a helm chart
// and install it to a kubernetes cluster.
package chart

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"helm.sh/helm/v3/pkg/chartutil"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/templates"
)

// ApplicationChart is an internal representation of a helm chart converted from the App CRD
// and is used to render a helm chart.
type ApplicationChart struct {
	values    values
	templates map[string]string
}

type values struct {
	App               *app                           `json:"app"`
	DockerRegistry    dockerRegistrySpec             `json:"dockerRegistry"`
	IngressController *ketchv1.IngressControllerSpec `json:"ingressController"`
}

// httpsEndpoint holds cname and its corresponding secret name with SSL certificates.
type httpsEndpoint struct {
	Cname string `json:"cname"`

	// SecretName is a name of a Kubernetes Secret to store SSL certificate for the cname.
	SecretName string `json:"secretName"`
}

// Ingress contains information about entrypoints of an application.
// Both istio and traefik templates use "ingress" to render Kubernetes Ingress objects.
type ingress struct {

	// Https is a list of http entrypoints.
	Http []string `json:"http"`

	// Https is a list of https entrypoints.
	Https []httpsEndpoint `json:"https"`
}

type app struct {
	Name        string        `json:"name"`
	Deployments []deployment  `json:"deployments"`
	Env         []ketchv1.Env `json:"env"`
	Ingress     ingress       `json:"ingress"`
	// IsAccessible if not set, ketch won't create kubernetes objects like Ingress/Gateway to handle incoming request.
	// These objects could be broken without valid routes to the application.
	// For example, "spec.rules" of an Ingress object must contain at least one rule.
	IsAccessible bool `json:"isAccessible"`
}

type deployment struct {
	Image           string                    `json:"image"`
	Version         ketchv1.DeploymentVersion `json:"version"`
	Processes       []process                 `json:"processes"`
	Labels          []ketchv1.Label           `json:"labels"`
	RoutingSettings ketchv1.RoutingSettings   `json:"routingSettings"`
	DeploymentExtra deploymentExtra           `json:"extra"`
}

type deploymentExtra struct {
	Volumes []v1.Volume `json:"volumes,omitempty"`
}

type dockerRegistrySpec struct {
	ImagePullSecret string `json:"imagePullSecret"`
}

type Option func(opts *Options)

type Options struct {
	// ExposedPorts are ports exposed by an image of each deployment.
	ExposedPorts map[ketchv1.DeploymentVersion][]ketchv1.ExposedPort
	Templates    templates.Templates
}

func WithExposedPorts(ports map[ketchv1.DeploymentVersion][]ketchv1.ExposedPort) Option {
	return func(opts *Options) {
		opts.ExposedPorts = make(map[ketchv1.DeploymentVersion][]ketchv1.ExposedPort, len(ports))
		for version, ps := range ports {
			if len(ps) == 0 {
				opts.ExposedPorts[version] = []ketchv1.ExposedPort{{Port: DefaultApplicationPort, Protocol: "TCP"}}
			} else {
				opts.ExposedPorts[version] = ps
			}
		}
	}
}

func WithTemplates(tpls templates.Templates) Option {
	return func(opts *Options) {
		opts.Templates = tpls
	}
}

// New returns an ApplicationChart instance.
func New(application *ketchv1.App, framework *ketchv1.Framework, opts ...Option) (*ApplicationChart, error) {

	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	values := &values{
		App: &app{
			Name:    application.Name,
			Ingress: newIngress(*application, *framework),
			Env:     application.Spec.Env,
		},
		IngressController: &framework.Spec.IngressController,
		DockerRegistry: dockerRegistrySpec{
			ImagePullSecret: application.Spec.DockerRegistry.SecretName,
		},
	}

	for _, deploymentSpec := range application.Spec.Deployments {
		deployment := deployment{
			Image:   deploymentSpec.Image,
			Version: deploymentSpec.Version,
			Labels:  deploymentSpec.Labels,
			RoutingSettings: ketchv1.RoutingSettings{
				Weight: deploymentSpec.RoutingSettings.Weight,
			},
		}
		procfile, err := ProcfileFromProcesses(deploymentSpec.Processes)
		if err != nil {
			return nil, err
		}
		exposedPorts := options.ExposedPorts[deployment.Version]
		c := NewConfigurator(deploymentSpec.KetchYaml, *procfile, exposedPorts, DefaultApplicationPort)
		for _, processSpec := range deploymentSpec.Processes {
			name := processSpec.Name
			isRoutable := procfile.IsRoutable(name)
			process, err := newProcess(name, isRoutable,
				withCmd(c.procfile.Processes[name]),
				withUnits(processSpec.Units),
				withPortsAndProbes(c),
				withLifecycle(c.Lifecycle()),
				withSecurityContext(processSpec.SecurityContext))

			if err != nil {
				return nil, err
			}

			deployment.Processes = append(deployment.Processes, *process)
		}
		values.App.Deployments = append(values.App.Deployments, deployment)
	}
	values.App.IsAccessible = isAppAccessible(values.App)
	return &ApplicationChart{
		values:    *values,
		templates: options.Templates.Yamls,
	}, nil
}

func (chrt ApplicationChart) getValuesMap() (map[string]interface{}, error) {
	bs, err := yaml.Marshal(chrt.values)
	if err != nil {
		return nil, err
	}
	vals, err := chartutil.ReadValues(bs)
	if err != nil {
		return nil, err
	}
	return vals, nil
}

const (
	chartYaml = `apiVersion: v2
name: {{ .AppName }}
description: {{ .Description }}
type: application
version: {{ .Version }}
{{- if .AppVersion }}
appVersion: {{ .AppVersion }}
{{- end }}
`
)

// ChartConfig contains data used to render the helm chart's "Chart.yaml" file.
type ChartConfig struct {
	Version     string
	Description string
	AppName     string
	AppVersion  string
}

// NewChartConfig returns a ChartConfig instance based on the given application.
func NewChartConfig(app ketchv1.App) ChartConfig {
	version := fmt.Sprintf("v%v", app.ObjectMeta.Generation)
	chartVersion := fmt.Sprintf("v0.0.%v", app.ObjectMeta.Generation)
	if app.Spec.Version != nil {
		version = *app.Spec.Version
	}
	return ChartConfig{
		Version:     chartVersion,
		Description: app.Spec.Description,
		AppName:     app.Name,
		AppVersion:  version,
	}
}

func (config ChartConfig) render() ([]byte, error) {
	buf := bytes.Buffer{}
	t := template.Must(template.New("chart.yaml").Parse(chartYaml))
	err := t.Execute(&buf, config)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ExportToDirectory saves the chart to the provided directory inside a folder with app_Name_TIMESTAMP
//  for example, for any app with name `hello`, it will save chart inside a folder with name `hello_11_Dec_20_12_30_IST`
func (chrt ApplicationChart) ExportToDirectory(directory string, chartConfig ChartConfig) error {
	timestamp := time.Now().Format(time.RFC822)
	replacer := strings.NewReplacer(" ", "_", ":", "_")
	chartDir := chartConfig.AppName + "_" + replacer.Replace(timestamp)
	targetDir := filepath.Join(directory, chartDir)

	err := os.MkdirAll(filepath.Join(targetDir, "templates"), os.ModePerm)
	if err != nil {
		return err
	}
	for filename, content := range chrt.templates {
		path := filepath.Join(targetDir, "templates", filename)
		err = ioutil.WriteFile(path, []byte(content), 0644)
		if err != nil {
			return err
		}
	}
	valuesBytes, err := yaml.Marshal(chrt.values)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath.Join(targetDir, "values.yaml"), valuesBytes, 0644)
	if err != nil {
		return err
	}
	chartYamlContent, err := chartConfig.render()
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath.Join(targetDir, "Chart.yaml"), chartYamlContent, 0644)
	if err != nil {
		return err
	}
	return nil
}

// GetName returns the app name, satisfying TemplateValuer
func (a ApplicationChart) GetName() string {
	return a.values.App.Name
}

// GetTemplates returns the app templates, satisfying TemplateValuer
func (a ApplicationChart) GetTemplates() map[string]string {
	return a.templates
}

// GetValues returns the app values, satisfying TemplateValuer
func (a ApplicationChart) GetValues() interface{} {
	return a.values
}

func isAppAccessible(a *app) bool {
	if len(a.Ingress.Http)+len(a.Ingress.Https) == 0 {
		return false
	}
	for _, deployment := range a.Deployments {
		for _, process := range deployment.Processes {
			if process.Routable {
				return true
			}
		}
	}
	return false
}

func newIngress(app ketchv1.App, framework ketchv1.Framework) ingress {
	var http []string
	var https []string
	if len(framework.Spec.IngressController.ClusterIssuer) > 0 {
		// cluster issuer is mandatory to obtain SSL certificates.
		https = app.Spec.Ingress.Cnames
	} else {
		http = app.Spec.Ingress.Cnames
	}
	var httpsEndpoints []httpsEndpoint
	for _, cname := range https {
		hash := sha256.New()
		hash.Write([]byte(fmt.Sprintf("cname-%s", cname)))
		bs := hash.Sum(nil)
		secretName := fmt.Sprintf("%s-cname-%x", app.Name, bs[:10])
		httpsEndpoints = append(httpsEndpoints, httpsEndpoint{Cname: cname, SecretName: secretName})
	}
	defaultCname := app.DefaultCname(&framework)
	if defaultCname != nil {
		http = append(http, *defaultCname)
	}
	return ingress{
		Http:  http,
		Https: httpsEndpoints,
	}
}
