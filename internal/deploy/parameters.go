package deploy

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/spf13/pflag"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/errors"
	"github.com/shipa-corp/ketch/internal/utils"
)

// Command line flags
const (
	FlagImage          = "image"
	FlagKetchYaml      = "ketch-yaml"
	FlagProcFile       = "procfile"
	FlagStrict         = "strict"
	FlagSteps          = "steps"
	FlagStepInterval   = "step-interval"
	FlagWait           = "wait"
	FlagTimeout        = "timeout"
	FlagDescription    = "description"
	FlagEnvironment    = "env"
	FlagFramework      = "framework"
	FlagRegistrySecret = "registry-secret"
	FlagBuilder        = "builder"
	FlagBuildPacks     = "build-packs"

	FlagImageShort       = "i"
	FlagDescriptionShort = "d"
	FlagEnvironmentShort = "e"
	FlagFrameworkShort   = "k"

	DefaultBuilder = "heroku/buildpacks:20"

	defaultYamlFile = "ketch.yaml"
)

// Services contains interfaces and function pointers to external services needed for deploy. The purpose of this
// structure is so that we can swap out implementations of these services for unit tests.
type Services struct {
	// Client gets updates and creates ketch CRDs
	Client Client
	// Kubernetes client
	KubeClient kubernetes.Interface
	// Builder references source builder from internal/builder package
	Builder SourceBuilderFn
	// Function that retrieve image config
	GetImageConfig GetImageConfigFn
	// Wait is a function that will wait until it detects the a deployment is finished
	Wait WaitFn
	// Writer probably points to stdout or stderr, receives textual output
	Writer io.Writer
}

// Options receive values set in flags.  They are processed into a ChangeSet
// which describes the values that have been explicitly set by the end user. In
// this way we know if we will need to update an existing app CRD.
type Options struct {
	AppName                 string
	Image                   string
	KetchYamlFileName       string
	ProcfileFileName        string
	StrictKetchYamlDecoding bool
	Steps                   int
	StepTimeInterval        string
	Wait                    bool
	Timeout                 string
	AppSourcePath           string
	SubPaths                []string

	Framework            string
	Description          string
	Envs                 []string
	DockerRegistrySecret string
	Builder              string
	BuildPacks           []string
}

type ChangeSet struct {
	appName              string
	yamlStrictDecoding   bool
	sourcePath           *string
	image                *string
	ketchYamlFileName    *string
	procfileFileName     *string
	steps                *int
	stepTimeInterval     *string
	wait                 *bool
	timeout              *string
	subPaths             *[]string
	platform             *string
	description          *string
	envs                 *[]string
	framework            *string
	dockerRegistrySecret *string
	builder              *string
	buildPacks           *[]string
}

func (o Options) GetChangeSet(flags *pflag.FlagSet) *ChangeSet {
	var cs ChangeSet
	cs.appName = o.AppName
	cs.yamlStrictDecoding = o.StrictKetchYamlDecoding

	// setting values for defaults we want to retain
	cs.timeout = &o.Timeout

	if o.AppSourcePath != "" {
		cs.sourcePath = &o.AppSourcePath
	}
	m := map[string]func(c *ChangeSet){
		FlagImage: func(c *ChangeSet) {
			c.image = &o.Image
		},
		FlagKetchYaml: func(c *ChangeSet) {
			c.ketchYamlFileName = &o.KetchYamlFileName
		},
		FlagProcFile: func(c *ChangeSet) {
			c.procfileFileName = &o.ProcfileFileName
		},
		FlagSteps: func(c *ChangeSet) {
			c.steps = &o.Steps
		},
		FlagStepInterval: func(c *ChangeSet) {
			c.stepTimeInterval = &o.StepTimeInterval
		},
		FlagWait: func(c *ChangeSet) {
			c.wait = &o.Wait
		},
		FlagTimeout: func(c *ChangeSet) {
			c.timeout = &o.Timeout
		},
		FlagDescription: func(c *ChangeSet) {
			c.description = &o.Description
		},
		FlagEnvironment: func(c *ChangeSet) {
			c.envs = &o.Envs
		},
		FlagFramework: func(c *ChangeSet) {
			c.framework = &o.Framework
		},
		FlagRegistrySecret: func(c *ChangeSet) {
			c.dockerRegistrySecret = &o.DockerRegistrySecret
		},
		FlagBuilder: func(c *ChangeSet) {
			c.builder = &o.Builder
		},
		FlagBuildPacks: func(c *ChangeSet) {
			c.buildPacks = &o.BuildPacks
		},
	}
	for k, f := range m {
		if flags.Changed(k) {
			f(&cs)
		}
	}
	return &cs
}

func (c *ChangeSet) getProcfileName() (string, error) {
	if c.procfileFileName == nil {
		return "", newMissingError(FlagProcFile)
	}
	return *c.procfileFileName, nil
}

func (c *ChangeSet) getDescription() (string, error) {
	if c.description == nil {
		return "", newMissingError(FlagDescription)
	}
	return *c.description, nil
}

func (c *ChangeSet) getYamlPath() (string, error) {
	if c.ketchYamlFileName == nil {
		return "", newMissingError(FlagKetchYaml)
	}
	stat, err := os.Stat(*c.ketchYamlFileName)
	if err != nil {
		return "", newInvalidError(FlagKetchYaml)
	}
	if stat.IsDir() {
		return "", fmt.Errorf("%w %s is not a regular file", newInvalidError(FlagKetchYaml), *c.ketchYamlFileName)
	}
	return *c.ketchYamlFileName, nil
}

func (c *ChangeSet) getSourceDirectory() (string, error) {
	if c.sourcePath == nil {
		return "", newMissingError("source directory")
	}
	if err := directoryExists(*c.sourcePath); err != nil {
		return "", err
	}
	return *c.sourcePath, nil
}

func (c *ChangeSet) getFramework(ctx context.Context, client Client) (string, error) {
	if c.framework == nil {
		return "", newMissingError(FlagFramework)
	}
	var p ketchv1.Framework
	err := client.Get(ctx, types.NamespacedName{Name: *c.framework}, &p)
	if apierrors.IsNotFound(err) {
		return "", fmt.Errorf("%w framework %q has not been created", newInvalidError(FlagFramework), *c.framework)
	}
	if err != nil {
		return "", errors.Wrap(err, "could not fetch framework %q", *c.framework)
	}
	return *c.framework, nil
}

func (c *ChangeSet) getImage() (string, error) {
	if c.image == nil {
		return "", fmt.Errorf("%w %s is required", newMissingError(FlagImage), FlagImage)
	}
	return *c.image, nil
}

func (c *ChangeSet) getSteps() (int, error) {
	if c.steps == nil {
		return 0, newMissingError(FlagSteps)
	}
	steps := *c.steps
	if steps < minimumSteps || steps > maximumSteps {
		return 0, fmt.Errorf("%w %s must be between %d and %d",
			newInvalidError(FlagSteps), FlagSteps, minimumSteps, maximumSteps)
	}

	return *c.steps, nil
}

func (c *ChangeSet) getStepInterval() (time.Duration, error) {
	if c.stepTimeInterval == nil {
		return 0, newMissingError(FlagStepInterval)
	}
	dur, err := time.ParseDuration(*c.stepTimeInterval)
	if err != nil {
		return 0, newInvalidError(FlagStepInterval)
	}
	return dur, nil
}

func (c *ChangeSet) getStepWeight() (uint8, error) {
	steps, err := c.getSteps()
	if err != nil {
		return 0, err
	}
	return uint8(steps / maximumSteps), nil
}

func (c *ChangeSet) getEnvironments() ([]ketchv1.Env, error) {
	if c.envs == nil {
		return nil, newMissingError(FlagEnvironment)
	}
	envs, err := utils.MakeEnvironments(*c.envs)
	if err != nil {
		return nil, newInvalidError(FlagEnvironment)
	}
	return envs, nil
}

func (c *ChangeSet) getWait() (bool, error) {
	if c.wait == nil {
		return false, newMissingError(FlagWait)
	}
	return *c.wait, nil
}

func (c *ChangeSet) getTimeout() (time.Duration, error) {
	if c.timeout == nil {
		return 0, newMissingError(FlagTimeout)
	}
	d, err := time.ParseDuration(*c.timeout)
	if err != nil {
		return 0, newInvalidError(FlagTimeout)
	}
	return d, nil
}

func (c *ChangeSet) getDockerRegistrySecret() (string, error) {
	if c.dockerRegistrySecret == nil {
		return "", newMissingError(FlagRegistrySecret)
	}
	return *c.dockerRegistrySecret, nil
}

// If the builder is assigned on the command we always use it.  Otherwise we look for a previously defined
// builder and use that if it exists, otherwise use the default builder.
func (c *ChangeSet) getBuilder(spec ketchv1.AppSpec) string {
	if c.builder == nil {
		if spec.Builder == "" {
			c.builder = func(s string) *string {
				return &s
			}(DefaultBuilder)
		} else {
			c.builder = &spec.Builder
		}
	}
	return *c.builder
}

func (c *ChangeSet) getBuildPacks() ([]string, error) {
	if c.buildPacks == nil {
		return nil, newMissingError(FlagBuildPacks)
	}
	return *c.buildPacks, nil
}

func (c *ChangeSet) getKetchYaml() (*ketchv1.KetchYamlData, error) {
	var fileName string
	// try to find yaml file in default location
	sourcePath, err := c.getSourceDirectory()
	if !isMissing(err) && isValid(err) {
		yamlPath := path.Join(sourcePath, defaultYamlFile)
		if stat, err := os.Stat(yamlPath); err == nil && !stat.IsDir() {
			fileName = yamlPath
		}
	}

	// if the yaml path is supplied on the  command line it takes precedence over
	// default yaml file
	yamlPath, err := c.getYamlPath()
	if !isMissing(err) && isValid(err) {
		fileName = yamlPath
	}

	// if no yaml is provided we're done
	if fileName == "" {
		return nil, nil
	}

	content, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	var decodeOpts []yaml.JSONOpt
	if c.yamlStrictDecoding {
		decodeOpts = append(decodeOpts, yaml.DisallowUnknownFields)
	}
	data := &ketchv1.KetchYamlData{}
	if err = yaml.Unmarshal(content, data, decodeOpts...); err != nil {
		return nil, err
	}
	return data, nil
}
