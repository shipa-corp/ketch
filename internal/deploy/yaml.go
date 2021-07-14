package deploy

import (
	"os"
	"strings"

	"github.com/shipa-corp/ketch/internal/utils"

	"github.com/shipa-corp/ketch/internal/errors"
	"sigs.k8s.io/yaml"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

// Application represents the fields in an application.yaml file that will be
// transitioned to a ChangeSet.
type Application struct {
	Version        *string    `json:"version"`
	Type           *string    `json:"type"`
	Name           *string    `json:"name"`
	Image          *string    `json:"image"`
	Framework      *string    `json:"framework"`
	Description    *string    `json:"description"`
	Environment    *[]string  `json:"environment"`
	RegistrySecret *string    `json:"registrySecret"`
	Builder        *string    `json:"builder"`
	BuildPacks     *[]string  `json:"buildPacks"`
	Processes      *[]Process `json:"processes"`
	CName          *CName     `json:"cname"`
	AppUnit        *int       `json:"appUnit"`
}

type Process struct {
	Name  string `json:"name"`  // required
	Cmd   string `json:"cmd"`   // required
	Units *int   `json:"units"` // unset? get from AppUnit
	Ports []Port `json:"ports"` // appDeploymentSpec
	Hooks Hooks  `json:"hooks"`
}

type Port struct {
	Protocol   string `json:"protocol"`
	Port       int    `json:"port"`
	TargetPort int    `json:"targetPort"`
}

type Hooks struct {
	Restart Restart `json:"restart"`
}

type Restart struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

type CName struct {
	DNSName string `json:"dnsName"`
}

var (
	defaultVersion  = "v1"
	defaultAppUnit  = 1
	typeApplication = "Application"
	typeJob         = "Job"
)

// GetChangeSetFromYaml reads an application.yaml file and returns a ChangeSet
// from the file's values.
func (o *Options) GetChangeSetFromYaml(filename string) (*ChangeSet, error) {
	var application Application
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(b, &application)
	if err != nil {
		return nil, err
	}

	var envs []ketchv1.Env
	if application.Environment != nil {
		envs, err = utils.MakeEnvironments(*application.Environment)
		if err != nil {
			return nil, err
		}
	}
	// processes, hooks, ports
	var processes []ketchv1.ProcessSpec
	var ketchYamlData ketchv1.KetchYamlData
	if application.Processes != nil {
		var beforeHooks []string
		var afterHooks []string
		ketchYamlProcessConfig := make(map[string]ketchv1.KetchYamlProcessConfig)
		for _, process := range *application.Processes {
			processes = append(processes, ketchv1.ProcessSpec{
				Name:  process.Name,
				Cmd:   strings.Split(process.Cmd, " "),
				Units: process.Units,
				Env:   envs,
			})
			if process.Hooks.Restart.Before != "" {
				beforeHooks = append(beforeHooks, process.Hooks.Restart.Before)
			}
			if process.Hooks.Restart.After != "" {
				afterHooks = append(afterHooks, process.Hooks.Restart.After)
			}

			var ports []ketchv1.KetchYamlProcessPortConfig
			for _, port := range process.Ports {
				ports = append(ports, ketchv1.KetchYamlProcessPortConfig{
					Protocol:   port.Protocol,
					Port:       port.Port,
					TargetPort: port.TargetPort,
				})
			}
			if len(process.Ports) > 0 {
				ketchYamlProcessConfig[process.Name] = ketchv1.KetchYamlProcessConfig{
					Ports: ports,
				}
			}
		}

		// assign hooks and ports (kubernetes processConfig) to ketch yaml data
		// NOTE: there is a disparity in that the yaml file format implies that hooks and ports
		// are per-process, while the AppSpec makes them per-deployment.
		ketchYamlData = ketchv1.KetchYamlData{
			Hooks: &ketchv1.KetchYamlHooks{
				Restart: ketchv1.KetchYamlRestartHooks{
					Before: beforeHooks,
					After:  afterHooks,
				},
			},
			Kubernetes: &ketchv1.KetchYamlKubernetesConfig{
				Processes: ketchYamlProcessConfig,
			},
		}
	}
	c := &ChangeSet{
		appName:              *application.Name,
		appVersion:           application.Version,
		appType:              application.Type,
		image:                application.Image,
		description:          application.Description,
		envs:                 application.Environment,
		framework:            application.Framework,
		dockerRegistrySecret: application.RegistrySecret,
		builder:              application.Builder,
		buildPacks:           application.BuildPacks,
		appUnit:              application.AppUnit,
		timeout:              &o.Timeout,
		wait:                 &o.Wait,
	}
	if o.AppSourcePath != "" {
		c.sourcePath = &o.AppSourcePath
	}
	if application.CName != nil {
		c.cname = &ketchv1.CnameList{application.CName.DNSName}
	}
	if len(processes) > 0 {
		c.processes = &processes
		c.ketchYamlData = &ketchYamlData
	}
	c.applyDefaults()
	return c, c.validate()
}

// apply defaults sets default values for a ChangeSet
func (c *ChangeSet) applyDefaults() {
	if c.appVersion == nil {
		c.appVersion = &defaultVersion
	}
	if c.appType == nil {
		c.appType = &typeApplication
	}
	c.yamlStrictDecoding = true

	// default to AppUnits if process.Units is unset
	if c.appUnit == nil {
		c.appUnit = &defaultAppUnit
	}
	if c.processes != nil {
		for i := range *c.processes {
			if (*c.processes)[i].Units == nil {
				(*c.processes)[i].Units = c.appUnit
			}
		}
	}
}

// validate assures that a ChangeSet's required fields are set
func (c *ChangeSet) validate() error {
	if c.framework == nil {
		return errors.New("missing required field framework")
	}
	if c.image == nil {
		return errors.New("missing required field image")
	}
	if c.appName == "" {
		return errors.New("missing required field name")
	}
	if c.sourcePath == nil && c.processes != nil {
		return errors.New("running defined processes require a sourcePath")
	}
	return nil
}
