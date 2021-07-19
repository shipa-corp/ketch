/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	"errors"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/shipa-corp/ketch/internal/templates"
)

func init() {
	SchemeBuilder.Register(&App{}, &AppList{})
}

const (
	ShipaCloudDomain     = "shipa.cloud"
	DefaultNumberOfUnits = 1
)

// Env represents an environment variable present in an application.
type Env struct {
	// +kubebuilder:validation:MinLength=1
	// Name of the environment variable. Must be a C_IDENTIFIER.
	Name string `json:"name"`

	// Value of the environment variable.
	Value string `json:"value"`
}

// Label represents an environment variable present in an application.
type Label struct {
	// +kubebuilder:validation:MinLength=1
	// Name of the label.
	Name string `json:"name"`

	// Value of the label.
	Value string `json:"value"`
}

// CnameList is a list of an app's CNAMEs.
type CnameList []string

// RoutingSettings contains a weight of the current deployment used to route incoming traffic.
// If an application has two deployments with corresponding weights of 30 and 70,
// then 3 of 10 incoming requests will be sent to the first deployment (approximately).
type RoutingSettings struct {
	Weight uint8 `json:"weight"`
}

// ProcessSpec is a specification of the desired behavior of a process.
type ProcessSpec struct {
	// +kubebuilder:validation:MinLength=1
	// Name of the process.
	Name string `json:"name"`

	// Units is a number of replicas of the process.
	Units *int `json:"units,omitempty"`

	// Env is a list of environment variables to set in pods created for the process.
	Env []Env `json:"env,omitempty"`

	// Commands executed on startup.
	Cmd []string `json:"cmd"`

	// Security options the process should run with.
	SecurityContext *v1.SecurityContext `json:"securityContext,omitempty"`
}

type DeploymentVersion int

func (v DeploymentVersion) String() string {
	return fmt.Sprintf("%d", v)
}

type AppDeploymentSpec struct {
	Image           string            `json:"image"`
	Version         DeploymentVersion `json:"version"`
	Processes       []ProcessSpec     `json:"processes,omitempty"`
	KetchYaml       *KetchYamlData    `json:"ketchYaml,omitempty"`
	Labels          []Label           `json:"labels,omitempty"`
	RoutingSettings RoutingSettings   `json:"routingSettings,omitempty"`
	ExposedPorts    []ExposedPort     `json:"exposedPorts,omitempty"`
}

// IngressSpec configures entrypoints to access an application.
type IngressSpec struct {

	// GenerateDefaultCname if set the application will have a default cname <app-name>.<ServiceEndpoint>.shipa.cloud.
	GenerateDefaultCname bool `json:"generateDefaultCname"`

	// Cnames is a list of additional cnames.
	Cnames CnameList `json:"cnames,omitempty"`
}

// DockerRegistrySpec contains docker registry configuration of an application.
type DockerRegistrySpec struct {

	// SecretName is added to the "imagePullSecrets" list of each application pod.
	SecretName string `json:"secretName,omitempty"`
}

// AppPhase is a label for the condition of an application at the current time.
type AppPhase string

const (
	// AppCreated means the app has been accepted by the system, but has not been started.
	AppCreated AppPhase = "Created"

	// AppError means the app CRD is broken in some way and ketch controller can't render and install a new helm chart.
	AppError AppPhase = "Error"

	// AppRunning means that ketch controller has rendered a helm chart of the application and installed it to a cluster.
	AppRunning AppPhase = "Running"
)

// AppStatus represents information about the status of an application.
type AppStatus struct {

	// Conditions of App resource.
	Conditions []Condition `json:"conditions,omitempty"`

	Framework *v1.ObjectReference `json:"framework,omitempty"`
}

// CanarySpec represents configuration for a canary deployment.
type CanarySpec struct {
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Steps int `json:"steps,omitempty"`
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	StepWeight      uint8         `json:"stepWeight,omitempty"`
	StepTimeInteval time.Duration `json:"stepTimeInterval,omitempty"`
	// NextScheduledTime holds time of the next step.
	NextScheduledTime *metav1.Time `json:"nextScheduledTime,omitempty"`
	// CurrentStep is the count for current step for a canary deployment.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	CurrentStep int `json:"currentStep,omitempty"`
	// Active shows if canary deployment is active for this application.
	Active bool `json:"active,omitempty"`
	// Started holds time when canary started
	Started *metav1.Time `json:"started,omitempty"`
}

// AppSpec defines the desired state of App.
type AppSpec struct {
	Version *string `json:"version,omitempty"`

	// +kubebuilder:validation:MaxLength=140
	Description string `json:"description,omitempty"`

	// Canary contains a configuration which will be required for canary deployments.
	Canary CanarySpec `json:"canary,omitempty"`

	// Deployments is a list of running deployments.
	Deployments []AppDeploymentSpec `json:"deployments"`

	// DeploymentsCount is incremented every time a new deployment is added to Deployments and used as a version for new deployments.
	DeploymentsCount int `json:"deploymentsCount,omitempty"`

	// List of environment variables of the application.
	Env []Env `json:"env,omitempty"`

	// Framework is a name of a Framework used to run the application.
	// +kubebuilder:validation:MinLength=1
	Framework string `json:"framework"`

	// Ingress contains configuration of entrypoints to access the application.
	Ingress IngressSpec `json:"ingress"`

	// DockerRegistry contains docker registry configuration of the application.
	DockerRegistry DockerRegistrySpec `json:"dockerRegistry,omitempty"`

	// Builder is the name of the builder used to build source code.
	Builder string `json:"builder,omitempty"`

	// BuildPacks is a list of build packs to use when building from source.
	BuildPacks []string `json:"buildPacks,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Framework",type=string,JSONPath=`.spec.Framework`
// +kubebuilder:printcolumn:name="Description",type=string,JSONPath=`.spec.description`

// App is the Schema for the apps API.
type App struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppSpec   `json:"spec,omitempty"`
	Status AppStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AppList contains a list of App.
type AppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []App `json:"items"`
}

func (s *AppDeploymentSpec) setUnits(process string, units int) error {
	for i, processSpec := range s.Processes {
		if processSpec.Name == process {
			s.Processes[i].Units = &units
			return nil
		}
	}
	return ErrProcessNotFound
}

func (s *AppDeploymentSpec) setUnitsForAllProcess(units int) {
	for i := range s.Processes {
		s.Processes[i].Units = &units
	}
}

// SetUnits set quantity of units of the specified processes.
func (app *App) SetUnits(selector Selector, units int) error {
	deploymentFound := false
	for _, deploymentSpec := range app.Spec.Deployments {
		if selector.DeploymentVersion != nil && *selector.DeploymentVersion != deploymentSpec.Version {
			continue
		}
		if selector.Process != nil {
			if err := deploymentSpec.setUnits(*selector.Process, units); err != nil {
				return err
			}
		} else {
			deploymentSpec.setUnitsForAllProcess(units)
		}
		deploymentFound = true
	}
	if selector.DeploymentVersion != nil && !deploymentFound {
		return ErrDeploymentNotFound
	}
	return nil
}

// SetEnvs extends the current list of environment variables with the provided list.
// If the current list has an env variable from the provided list, the env variable will be updated with a new value.
func (app *App) SetEnvs(envs []Env) {
	names := make(map[string]Env, len(envs))
	for _, env := range envs {
		names[env.Name] = env
	}
	newEnvs := make([]Env, 0, len(envs))
	for _, env := range app.Spec.Env {
		if newEnv, hasNewValue := names[env.Name]; hasNewValue {
			newEnvs = append(newEnvs, newEnv)
			delete(names, env.Name)
			continue
		}
		newEnvs = append(newEnvs, env)
	}
	for _, env := range names {
		newEnvs = append(newEnvs, env)
	}
	app.Spec.Env = newEnvs
}

// Envs returns values of the asked env variables.
func (app *App) Envs(names []string) map[string]string {
	namesMap := make(map[string]struct{}, len(names))
	for _, name := range names {
		namesMap[name] = struct{}{}
	}

	envs := make(map[string]string)
	for _, env := range app.Spec.Env {
		if len(names) == 0 {
			envs[env.Name] = env.Value
			continue
		}
		if _, ok := namesMap[env.Name]; ok {
			envs[env.Name] = env.Value
		}
	}
	return envs
}

// UnsetEnvs unsets environment values.
func (app *App) UnsetEnvs(envs []string) {
	names := make(map[string]struct{}, len(envs))
	for _, name := range envs {
		names[name] = struct{}{}
	}
	var newEnvs []Env
	for _, env := range app.Spec.Env {
		if _, remove := names[env.Name]; !remove {
			newEnvs = append(newEnvs, env)
		}
	}
	app.Spec.Env = newEnvs
}

// Stop stops processes specified by the selector.
func (app *App) Stop(selector Selector) error {
	return app.SetUnits(selector, 0)
}

// Start starts processes specified by the selector.
// We start a process by setting its unit quantity to 1.
// If a process has running units, nothing will be changed.
func (app *App) Start(selector Selector) error {
	deploymentFound := false
	units := 1
	for _, deploymentSpec := range app.Spec.Deployments {
		if selector.DeploymentVersion != nil && *selector.DeploymentVersion != deploymentSpec.Version {
			continue
		}
		if selector.Process != nil {
			for i, processSpec := range deploymentSpec.Processes {
				if processSpec.Name == *selector.Process && (processSpec.Units == nil || *processSpec.Units == 0) {
					deploymentSpec.Processes[i].Units = &units
				}
			}
		} else {
			for i, processSpec := range deploymentSpec.Processes {
				if processSpec.Units != nil && *processSpec.Units > 1 {
					continue
				}
				deploymentSpec.Processes[i].Units = &units
			}
		}
		deploymentFound = true
	}
	if selector.DeploymentVersion != nil && !deploymentFound {
		return ErrDeploymentNotFound
	}
	return nil
}

// CNames returns all CNAMEs to access the application including a default cname.
func (app *App) CNames(framework *Framework) []string {
	scheme := "http"
	if len(framework.Spec.IngressController.ClusterIssuer) > 0 {
		scheme = "https"
	}
	cnames := []string{}
	defaultCname := app.DefaultCname(framework)
	if defaultCname != nil {
		cnames = append(cnames, fmt.Sprintf("http://%s", *defaultCname))
	}
	for _, cname := range app.Spec.Ingress.Cnames {
		cnames = append(cnames, fmt.Sprintf("%s://%s", scheme, cname))
	}
	return cnames
}

// DefaultCname returns a default cname to access the application.
// A default cname uses the following format: <app name>.<Framework's ServiceEndpoint>.shipa.cloud.
func (app *App) DefaultCname(framework *Framework) *string {
	if framework == nil {
		return nil
	}
	if !app.Spec.Ingress.GenerateDefaultCname {
		return nil
	}
	if len(framework.Spec.IngressController.ServiceEndpoint) == 0 {
		return nil
	}
	url := fmt.Sprintf("%s.%s.%s", app.Name, framework.Spec.IngressController.ServiceEndpoint, ShipaCloudDomain)
	return &url
}

// TemplatesConfigMapName returns a name of a configmap that contains templates used to render a helm chart.
func (app *App) TemplatesConfigMapName(ingressControllerType IngressControllerType) string {
	return templates.IngressConfigMapName(ingressControllerType.String())
}

// Units returns a total number units.
func (app *App) Units() int {
	units := 0
	for _, deploymentSpec := range app.Spec.Deployments {
		for _, processSpec := range deploymentSpec.Processes {
			if processSpec.Units == nil {
				units += 1
			} else {
				units += *processSpec.Units
			}
		}
	}
	return units
}

// ExposedPorts returns ports exposed by an image of each deployment.
func (app *App) ExposedPorts() map[DeploymentVersion][]ExposedPort {
	ports := make(map[DeploymentVersion][]ExposedPort, len(app.Spec.Deployments))
	for _, deployment := range app.Spec.Deployments {
		ports[deployment.Version] = deployment.ExposedPorts
	}
	return ports
}

// SetCondition sets Status and message fields of the given type of condition to the provided values.
func (app *App) SetCondition(t ConditionType, status v1.ConditionStatus, message string, time metav1.Time) {
	c := Condition{
		Type:               t,
		Status:             status,
		LastTransitionTime: &time,
		Message:            message,
	}
	for i, cond := range app.Status.Conditions {
		if cond.Type == t {
			if cond.Status == c.Status && cond.Message == c.Message {
				return
			}
			app.Status.Conditions[i] = c
			return
		}
	}
	app.Status.Conditions = append(app.Status.Conditions, c)
}

// Phase return a simple, high-level summary of where the application is in its lifecycle.
func (app *App) Phase() AppPhase {
	for _, cond := range app.Status.Conditions {
		if cond.Status == v1.ConditionFalse {
			return AppError
		}
	}
	if app.Units() == 0 {
		return AppCreated
	}
	return AppRunning
}

// Condition looks for a condition with the provided type in the condition list and returns it.
func (s AppStatus) Condition(t ConditionType) *Condition {
	for _, c := range s.Conditions {
		if c.Type == t {
			return &c
		}
	}
	return nil
}

// DoCanary checks if canary deployment is needed for an app and gradually increases the traffic weight
// based on the canary parameters provided by the users. Use it in app controller.
func (app *App) DoCanary(now metav1.Time) error {

	if !app.Spec.Canary.Active {
		return nil
	}

	if len(app.Spec.Deployments) <= 1 {
		return errors.New("no canary deployment found")
	}

	if app.Spec.Canary.NextScheduledTime == nil {
		return errors.New("canary is active but the next step is not scheduled")
	}

	if app.Spec.Canary.NextScheduledTime.Equal(&now) || app.Spec.Canary.NextScheduledTime.Before(&now) {
		// update traffic weight distributions across deployments
		app.Spec.Deployments[0].RoutingSettings.Weight = app.Spec.Deployments[0].RoutingSettings.Weight - app.Spec.Canary.StepWeight
		app.Spec.Deployments[1].RoutingSettings.Weight = app.Spec.Deployments[1].RoutingSettings.Weight + app.Spec.Canary.StepWeight
		app.Spec.Canary.CurrentStep++

		// update next scheduled time
		*app.Spec.Canary.NextScheduledTime = metav1.NewTime(app.Spec.Canary.NextScheduledTime.Add(app.Spec.Canary.StepTimeInteval))

		// check if the canary weight is exceeding 100% of traffic
		if app.Spec.Deployments[1].RoutingSettings.Weight >= 100 || app.Spec.Canary.CurrentStep == app.Spec.Canary.Steps {

			// we need to set weight of the target deployment to 100
			// because there is a chance that on the last step weight is not equal to 100 (e.g. steps=3, step-weight=33)
			app.Spec.Deployments[1].RoutingSettings.Weight = 100

			app.Spec.Canary.Active = false
			app.Spec.Canary.CurrentStep = app.Spec.Canary.Steps
			app.Spec.Canary.NextScheduledTime = nil

			// remove the primary deployment
			app.Spec.Deployments = []AppDeploymentSpec{app.Spec.Deployments[1]}
		}
	}

	return nil
}

// DoRollback performs rollback
func (app *App) DoRollback() {
	// we need to rollback all weight to the primary deployment
	app.Spec.Deployments[0].RoutingSettings.Weight = 100
	app.Spec.Deployments[1].RoutingSettings.Weight = 0
	app.Spec.Canary.Active = false
}

// PodState describes the simplified state of a pod in the cluster
type PodState string

const (
	// PodRunning means that pod running on the cluster
	PodRunning PodState = "running"
	// PodDeploying means that pod is creating on the cluster, it is not in running or error state
	PodDeploying PodState = "deploying"
	// PodError means that the pod is not in a healthy state, and action from the user may be needed
	PodError PodState = "error"
	// PodSucceeded means that all containers in the pod have voluntarily terminated
	// with a container exit code of 0, and the system is not going to restart any of these containers.
	PodSucceeded PodState = "succeeded"
)
