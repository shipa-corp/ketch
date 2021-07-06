package deploy

import (
	"os"
	"strings"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"

	"github.com/shipa-corp/ketch/internal/errors"
	"sigs.k8s.io/yaml"
)

type Application struct {
	Version        string    `json:"version"` // TODO - store on ketchv1.App
	Type           string    `json:"type"`    // TODO - determines App or Job
	Name           string    `json:"name"`
	Image          string    `json:"image"`
	Framework      string    `json:"framework"`
	Description    string    `json:"description"`
	Environment    []string  `json:"environment"`
	RegistrySecret string    `json:"registrySecret"`
	Builder        string    `json:"builder"`
	BuildPacks     []string  `json:"buildPacks"`
	Processes      []Process `json:"processes"` // TODO
	CName          CName     `json:"cname"`     // TODO
	AppUnit        int       `json:"appUnit"`   // TODO
}

type Process struct {
	Name  string `json:"name"`  // required
	Cmd   string `json:"cmd"`   // required
	Units int    `json:"units"` // unset? get from AppUnit
	Ports []Port `json:"ports"` // appDeploymentSpec
	Hooks []Hook `json:"hooks"`
}

type Port struct {
	Protocol   string `json:"protocol"`
	Port       int    `json:"port"`
	TargetPort int    `json:"targetPort"`
}

type Hook struct {
	Restart Restart `json:"restart"`
}

type Restart struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

type CName struct {
	DNSName string `json:"dnsName"`
	Secure  bool   `json:"secure"`
}

var (
	defaultVersion  = "v1"
	defaultAppUnit  = 1
	typeApplication = "Application"
	typeJob         = "Job"
)

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
	for _, env := range application.Environment {
		arr := strings.Split(env, "=")
		if len(arr) != 2 {
			continue
		}
		envs = append(envs, ketchv1.Env{Name: arr[0], Value: arr[1]})
	}
	// processes, hooks, ports
	var processes []ketchv1.ProcessSpec
	var beforeHooks []string
	var afterHooks []string
	ketchYamlProcessConfig := make(map[string]ketchv1.KetchYamlProcessConfig)
	for i, process := range application.Processes {
		processes = append(processes, ketchv1.ProcessSpec{
			Name:  process.Name,
			Cmd:   strings.Split(process.Cmd, " "),
			Units: &application.Processes[i].Units,
			Env:   envs,
		})
		for _, hook := range process.Hooks {
			beforeHooks = append(beforeHooks, hook.Restart.Before)
			afterHooks = append(afterHooks, hook.Restart.After)
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

	// assign hooks and ports (kubernetes processconfig) to ketch yaml data
	ketchYamlData := &ketchv1.KetchYamlData{
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
	c := &ChangeSet{
		appName:              application.Name,
		version:              &application.Version,
		appType:              &application.Type,
		yamlStrictDecoding:   true,
		image:                &application.Image,
		description:          &application.Description,
		envs:                 &application.Environment,
		framework:            &application.Framework,
		dockerRegistrySecret: &application.RegistrySecret,
		builder:              &application.Builder,
		buildPacks:           &application.BuildPacks,
		processes:            &processes,
		ketchYamlData:        ketchYamlData,
	}
	c.applyDefaults()
	return c, c.validate()
}

func (c *ChangeSet) applyDefaults() {
	if c.version == nil {
		c.version = &defaultVersion
	}
	if c.appType == nil {
		c.appType = &typeApplication
	}
	if c.appUnit == nil {
		c.appUnit = &defaultAppUnit
	}
	// building from source in PWD
	if c.builder != nil {
		sourcePath := "."
		c.sourcePath = &sourcePath
	}
}

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
	return nil
}
