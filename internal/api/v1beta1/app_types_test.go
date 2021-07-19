package v1beta1

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func intRef(i int) *int {
	return &i
}

func versionRef(i DeploymentVersion) *DeploymentVersion {
	return &i
}

func stringRef(s string) *string {
	return &s
}

func defaultSpec() AppSpec {
	return AppSpec{
		Deployments: []AppDeploymentSpec{
			{
				Version: 1,
				Processes: []ProcessSpec{
					{Name: "web", Units: intRef(1)},
					{Name: "worker", Units: intRef(2)},
				},
			},
			{
				Version: 2,
				Processes: []ProcessSpec{
					{Name: "web", Units: intRef(0)},
					{Name: "worker", Units: intRef(0)},
				},
			},
		},
	}
}

func TestApp_SetUnits(t *testing.T) {
	tests := []struct {
		name string

		spec     AppSpec
		selector Selector

		wantSpec AppSpec
		wantErr  error
	}{
		{
			name:     "empty selector, process not found",
			spec:     defaultSpec(),
			selector: Selector{Process: stringRef("database")},
			wantErr:  ErrProcessNotFound,
		},
		{
			name:     "empty selector, deployment not found",
			spec:     defaultSpec(),
			selector: Selector{DeploymentVersion: versionRef(8)},
			wantErr:  ErrDeploymentNotFound,
		},
		{
			name:     "empty selector, all processes of all deployments",
			spec:     defaultSpec(),
			selector: Selector{},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(4)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(4)},
						},
					},
				},
			},
		},
		{
			name:     "filter by process",
			spec:     defaultSpec(),
			selector: Selector{Process: stringRef("web")},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(2)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(0)},
						},
					},
				},
			},
		},
		{
			name:     "filter by deployment",
			spec:     defaultSpec(),
			selector: Selector{DeploymentVersion: versionRef(2)},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(1)},
							{Name: "worker", Units: intRef(2)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(4)},
						},
					},
				},
			},
		},
		{
			name:     "filter by deployment and process",
			spec:     defaultSpec(),
			selector: Selector{DeploymentVersion: versionRef(2), Process: stringRef("worker")},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(1)},
							{Name: "worker", Units: intRef(2)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(0)},
							{Name: "worker", Units: intRef(4)},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{Spec: tt.spec}
			if err := app.SetUnits(tt.selector, 4); err != tt.wantErr {
				t.Errorf("SetUnits() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr != nil {
				return
			}
			if diff := cmp.Diff(app.Spec, tt.wantSpec); diff != "" {
				t.Errorf("AppSpec mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApp_SetEnvs(t *testing.T) {
	tests := []struct {
		name        string
		initialEnvs []Env
		setEnvs     []Env
		wantEnvs    map[string]Env
	}{
		{
			name: "no new variables",
			initialEnvs: []Env{
				{Name: "KETCH", Value: "true"},
				{Name: "THEKETCH", Value: "true"},
				{Name: "API_KEY", Value: "xxx"},
			},
			setEnvs: []Env{},
			wantEnvs: map[string]Env{
				"KETCH":    {Name: "KETCH", Value: "true"},
				"THEKETCH": {Name: "THEKETCH", Value: "true"},
				"API_KEY":  {Name: "API_KEY", Value: "xxx"},
			},
		},
		{
			name: "partially update",
			initialEnvs: []Env{
				{Name: "KETCH", Value: "true"},
				{Name: "THEKETCH", Value: "true"},
				{Name: "API_KEY", Value: "xxx"},
			},
			setEnvs: []Env{
				{Name: "KETCH", Value: "false"},
				{Name: "THEKETCH", Value: "1"},
			},
			wantEnvs: map[string]Env{
				"KETCH":    {Name: "KETCH", Value: "false"},
				"THEKETCH": {Name: "THEKETCH", Value: "1"},
				"API_KEY":  {Name: "API_KEY", Value: "xxx"},
			},
		},
		{
			name:        "update all",
			initialEnvs: []Env{},
			setEnvs: []Env{
				{Name: "KETCH", Value: "false"},
				{Name: "THEKETCH", Value: "1"},
			},
			wantEnvs: map[string]Env{
				"KETCH":    {Name: "KETCH", Value: "false"},
				"THEKETCH": {Name: "THEKETCH", Value: "1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := App{
				Spec: AppSpec{
					Env: tt.initialEnvs,
				},
			}
			app.SetEnvs(tt.setEnvs)
			got := make(map[string]Env, len(app.Spec.Env))
			for _, env := range app.Spec.Env {
				got[env.Name] = env
			}
			if diff := cmp.Diff(got, tt.wantEnvs); diff != "" {
				t.Errorf("Envs mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApp_Envs(t *testing.T) {
	tests := []struct {
		name        string
		want        map[string]string
		initialEnvs []Env
		names       []string
	}{
		{
			name: "all the asked envs are present",
			initialEnvs: []Env{
				{Name: "KETCH", Value: "true"},
				{Name: "THEKETCH", Value: "true"},
				{Name: "API_KEY", Value: "xxx"},
			},
			names: []string{"KETCH", "API_KEY"},
			want: map[string]string{
				"KETCH":   "true",
				"API_KEY": "xxx",
			},
		},
		{
			name: "some of the asked envs are present",
			initialEnvs: []Env{
				{Name: "KETCH", Value: "true"},
			},
			names: []string{"KETCH", "API_KEY", "SOME_VAR"},
			want: map[string]string{
				"KETCH": "true",
			},
		},
		{
			name:  "app has no envs",
			names: []string{"KETCH", "API_KEY", "SOME_VAR"},
			want:  map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				Spec: AppSpec{
					Env: tt.initialEnvs,
				},
			}
			if got := app.Envs(tt.names); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Envs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApp_UnsetEnvs(t *testing.T) {
	tests := []struct {
		name        string
		initialEnvs []Env
		unset       []string
		wantEnvs    map[string]Env
	}{
		{
			name: "unset all",
			initialEnvs: []Env{
				{Name: "KETCH", Value: "true"},
				{Name: "THEKETCH", Value: "true"},
				{Name: "API_KEY", Value: "xxx"},
			},
			unset:    []string{"KETCH", "THEKETCH", "API_KEY", "SOME_KEY"},
			wantEnvs: map[string]Env{},
		},
		{
			name: "unset partially",
			initialEnvs: []Env{
				{Name: "KETCH", Value: "true"},
				{Name: "THEKETCH", Value: "true"},
				{Name: "API_KEY", Value: "xxx"},
			},
			unset: []string{"KETCH", "SOME_KEY"},
			wantEnvs: map[string]Env{
				"THEKETCH": {Name: "THEKETCH", Value: "true"},
				"API_KEY":  {Name: "API_KEY", Value: "xxx"},
			},
		},
		{
			name:        "app has no envs",
			initialEnvs: nil,
			unset:       []string{"KETCH", "SOME_KEY"},
			wantEnvs:    map[string]Env{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				Spec: AppSpec{
					Env: tt.initialEnvs,
				},
			}
			app.UnsetEnvs(tt.unset)
			got := make(map[string]Env, len(app.Spec.Env))
			for _, env := range app.Spec.Env {
				got[env.Name] = env
			}
			if diff := cmp.Diff(got, tt.wantEnvs); diff != "" {
				t.Errorf("Envs mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApp_DefaultCname(t *testing.T) {
	tests := []struct {
		name                 string
		appName              string
		generateDefaultCname bool
		framework            *Framework
		want                 *string
	}{
		{
			name:                 "cname with default domain",
			appName:              "app-2",
			generateDefaultCname: true,
			framework: &Framework{
				Spec: FrameworkSpec{
					IngressController: IngressControllerSpec{
						ServiceEndpoint: "20.20.20.20",
					},
				},
			},
			want: stringRef("app-2.20.20.20.20.shipa.cloud"),
		},
		{
			name:                 "no service endpoint - no default cname",
			appName:              "app-1",
			generateDefaultCname: true,
			framework:            &Framework{},
		},
		{
			name:                 "do not generate default cname - no default cname",
			appName:              "app-1",
			generateDefaultCname: false,
			framework: &Framework{
				Spec: FrameworkSpec{
					IngressController: IngressControllerSpec{
						ServiceEndpoint: "20.20.20.20",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				ObjectMeta: metav1.ObjectMeta{
					Name: tt.appName,
				},
				Spec: AppSpec{
					Ingress: IngressSpec{
						GenerateDefaultCname: tt.generateDefaultCname,
					},
				},
			}
			got := app.DefaultCname(tt.framework)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("DefaultCname() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApp_CNames(t *testing.T) {
	framework := Framework{
		Spec: FrameworkSpec{
			IngressController: IngressControllerSpec{
				ServiceEndpoint: "10.20.30.40",
			},
		},
	}
	frameworkWithClusterIssuer := Framework{
		Spec: FrameworkSpec{
			IngressController: IngressControllerSpec{
				ServiceEndpoint: "10.20.30.40",
				ClusterIssuer:   "letsencrypt",
			},
		},
	}
	tests := []struct {
		name                 string
		generateDefaultCname bool
		framework            Framework
		cnames               []string
		want                 []string
	}{
		{
			name:                 "with default cname",
			generateDefaultCname: true,
			framework:            framework,
			cnames:               []string{"theketch.io", "app.theketch.io"},
			want:                 []string{"http://ketch.10.20.30.40.shipa.cloud", "http://theketch.io", "http://app.theketch.io"},
		},
		{
			name:                 "with default cname and framework with cluster issuer",
			generateDefaultCname: true,
			framework:            frameworkWithClusterIssuer,
			cnames:               []string{"theketch.io", "app.theketch.io"},
			want:                 []string{"http://ketch.10.20.30.40.shipa.cloud", "https://theketch.io", "https://app.theketch.io"},
		},
		{
			name:                 "without default cname",
			generateDefaultCname: false,
			framework:            framework,
			cnames:               []string{"theketch.io", "app.theketch.io"},
			want:                 []string{"http://theketch.io", "http://app.theketch.io"},
		},
		{
			name:                 "empty cnames",
			framework:            framework,
			generateDefaultCname: false,
			want:                 []string{},
		},
		{
			name:                 "empty cnames with default cname",
			framework:            framework,
			generateDefaultCname: true,
			want:                 []string{"http://ketch.10.20.30.40.shipa.cloud"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ketch",
				},
				Spec: AppSpec{
					Ingress: IngressSpec{
						GenerateDefaultCname: tt.generateDefaultCname,
						Cnames:               tt.cnames,
					},
				},
			}
			got := app.CNames(&tt.framework)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("CNames() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApp_TemplatesConfigMapName(t *testing.T) {
	tests := []struct {
		name        string
		app         App
		ingressType IngressControllerType

		want string
	}{
		{
			name:        "istio configmap",
			app:         App{},
			ingressType: IstioIngressControllerType,
			want:        "ingress-istio-templates",
		},
		{
			name:        "traefik configmap",
			app:         App{},
			ingressType: TraefikIngressControllerType,
			want:        "ingress-traefik-templates",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.app.TemplatesConfigMapName(tt.ingressType)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("TemplatesConfigMapName() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApp_Units(t *testing.T) {
	tests := []struct {
		name string
		app  App
		want int
	}{
		{
			name: "app without units",
			app: App{
				Spec: AppSpec{
					Deployments: []AppDeploymentSpec{
						{
							Version:   1,
							Processes: []ProcessSpec{},
						},
					},
				},
			},
			want: 0,
		},
		{
			name: "app with units",
			app: App{
				Spec: AppSpec{
					Deployments: []AppDeploymentSpec{
						{
							Version: 1,
							Processes: []ProcessSpec{
								{Name: "web", Units: intRef(1)},
								{Name: "worker", Units: intRef(2)},
							},
						},
						{
							Version: 2,
							Processes: []ProcessSpec{
								{Name: "web", Units: intRef(8)},
								{Name: "worker", Units: intRef(10)},
							},
						},
					},
				},
			},
			want: 21,
		},
		{
			name: "each process has default number of units",
			app: App{
				Spec: AppSpec{
					Deployments: []AppDeploymentSpec{
						{Version: 1, Processes: []ProcessSpec{{Name: "web"}, {Name: "worker"}}},
						{Version: 2, Processes: []ProcessSpec{{Name: "web"}, {Name: "worker"}}},
					},
				},
			},
			want: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.app.Units(); got != tt.want {
				t.Errorf("Units() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApp_Stop(t *testing.T) {
	tests := []struct {
		name     string
		selector Selector

		wantSpec AppSpec
		wantErr  bool
	}{
		{
			name:     "stop by process",
			selector: Selector{Process: stringRef("web")},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(0)},
							{Name: "worker", Units: intRef(2)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(0)},
							{Name: "worker", Units: intRef(5)},
						},
					},
				},
			},
		},
		{
			name:     "stop by deployment",
			selector: Selector{DeploymentVersion: versionRef(1)},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(0)},
							{Name: "worker", Units: intRef(0)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(5)},
						},
					},
				},
			},
		},
		{
			name:     "stop by deployment and process",
			selector: Selector{DeploymentVersion: versionRef(2), Process: stringRef("worker")},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(1)},
							{Name: "worker", Units: intRef(2)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(0)},
						},
					},
				},
			},
		},
		{
			name:     "stop all",
			selector: Selector{},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(0)},
							{Name: "worker", Units: intRef(0)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(0)},
							{Name: "worker", Units: intRef(0)},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				Spec: AppSpec{
					Deployments: []AppDeploymentSpec{
						{
							Version: 1,
							Processes: []ProcessSpec{
								{Name: "web", Units: intRef(1)},
								{Name: "worker", Units: intRef(2)},
							},
						},
						{
							Version: 2,
							Processes: []ProcessSpec{
								{Name: "web", Units: intRef(4)},
								{Name: "worker", Units: intRef(5)},
							},
						},
					},
				},
			}
			if err := app.Stop(tt.selector); (err != nil) != tt.wantErr {
				t.Errorf("Stop() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if diff := cmp.Diff(app.Spec, tt.wantSpec); diff != "" {
				t.Errorf("AppSpec mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApp_Start(t *testing.T) {
	tests := []struct {
		name     string
		selector Selector

		wantSpec AppSpec
		wantErr  bool
	}{
		{
			name:     "start all",
			selector: Selector{},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(1)},
							{Name: "worker", Units: intRef(2)},
							{Name: "db", Units: intRef(1)},
							{Name: "db-2", Units: intRef(1)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(5)},
							{Name: "db", Units: intRef(1)},
							{Name: "db-2", Units: intRef(1)},
						},
					},
				},
			},
		},
		{
			name:     "start by process",
			selector: Selector{Process: stringRef("db")},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(1)},
							{Name: "worker", Units: intRef(2)},
							{Name: "db", Units: intRef(1)},
							{Name: "db-2", Units: intRef(0)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(5)},
							{Name: "db", Units: intRef(1)},
							{Name: "db-2", Units: intRef(0)},
						},
					},
				},
			},
		},
		{
			name:     "start by deployment",
			selector: Selector{DeploymentVersion: versionRef(2)},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(1)},
							{Name: "worker", Units: intRef(2)},
							{Name: "db"},
							{Name: "db-2", Units: intRef(0)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(5)},
							{Name: "db", Units: intRef(1)},
							{Name: "db-2", Units: intRef(1)},
						},
					},
				},
			},
		},
		{
			name:     "start by deployment and process",
			selector: Selector{DeploymentVersion: versionRef(1), Process: stringRef("db")},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(1)},
							{Name: "worker", Units: intRef(2)},
							{Name: "db", Units: intRef(1)},
							{Name: "db-2", Units: intRef(0)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(5)},
							{Name: "db"},
							{Name: "db-2", Units: intRef(0)},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				Spec: AppSpec{
					Deployments: []AppDeploymentSpec{
						{
							Version: 1,
							Processes: []ProcessSpec{
								{Name: "web", Units: intRef(1)},
								{Name: "worker", Units: intRef(2)},
								{Name: "db"},
								{Name: "db-2", Units: intRef(0)},
							},
						},
						{
							Version: 2,
							Processes: []ProcessSpec{
								{Name: "web", Units: intRef(4)},
								{Name: "worker", Units: intRef(5)},
								{Name: "db"},
								{Name: "db-2", Units: intRef(0)},
							},
						},
					},
				},
			}
			err := app.Start(tt.selector)
			if tt.wantErr {
				require.NotNil(t, err)
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.wantSpec, app.Spec)
		})
	}
}

func TestApp_Phase(t *testing.T) {
	tests := []struct {
		name string
		app  App
		want AppPhase
	}{
		{
			name: "1 unit - status is running",
			app: App{
				Spec: AppSpec{
					Deployments: []AppDeploymentSpec{
						{Processes: []ProcessSpec{{Units: intRef(1)}}},
					},
				},
				Status: AppStatus{
					Conditions: []Condition{
						{Type: Scheduled, Status: v1.ConditionTrue},
					},
				},
			},
			want: AppRunning,
		},
		{
			name: "schedule issue - status is error",
			app: App{
				Spec: AppSpec{
					Deployments: []AppDeploymentSpec{
						{Processes: []ProcessSpec{{Units: intRef(1)}}},
					},
				},
				Status: AppStatus{
					Conditions: []Condition{
						{Type: Scheduled, Status: v1.ConditionFalse},
					},
				},
			},
			want: AppError,
		},
		{
			name: "no units - status is created",
			app: App{
				Spec: AppSpec{},
				Status: AppStatus{
					Conditions: []Condition{
						{Type: Scheduled, Status: v1.ConditionTrue},
					},
				},
			},
			want: AppCreated,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase := tt.app.Phase()
			require.Equal(t, tt.want, phase)
		})
	}
}

func TestApp_SetCondition(t *testing.T) {

	t1 := metav1.NewTime(time.Now())
	t2 := metav1.NewTime(time.Now())

	tests := []struct {
		name              string
		currentConditions []Condition
		wantConditions    []Condition
	}{
		{
			name:              "add condition",
			currentConditions: nil,
			wantConditions: []Condition{
				{Type: Scheduled, Status: v1.ConditionTrue, LastTransitionTime: &t2, Message: "message"},
			},
		},
		{
			name:              "message updated",
			currentConditions: []Condition{{Type: Scheduled, Status: v1.ConditionTrue, LastTransitionTime: &t1, Message: "old-message"}},
			wantConditions: []Condition{
				{Type: Scheduled, Status: v1.ConditionTrue, LastTransitionTime: &t2, Message: "message"},
			},
		},
		{
			name:              "status updated",
			currentConditions: []Condition{{Type: Scheduled, Status: v1.ConditionFalse, LastTransitionTime: &t1, Message: "message"}},
			wantConditions: []Condition{
				{Type: Scheduled, Status: v1.ConditionTrue, LastTransitionTime: &t2, Message: "message"},
			},
		},
		{
			name:              "no need to update",
			currentConditions: []Condition{{Type: Scheduled, Status: v1.ConditionTrue, LastTransitionTime: &t1, Message: "message"}},
			wantConditions: []Condition{
				{Type: Scheduled, Status: v1.ConditionTrue, LastTransitionTime: &t1, Message: "message"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := App{
				Status: AppStatus{
					Conditions: tt.currentConditions,
				},
			}
			condition := Condition{
				Type:               Scheduled,
				Status:             v1.ConditionTrue,
				LastTransitionTime: &t2,
				Message:            "message",
			}
			app.SetCondition(condition.Type, condition.Status, condition.Message, *condition.LastTransitionTime)
			require.Equal(t, tt.wantConditions, app.Status.Conditions)
		})
	}
}

func TestApp_DoCanary(t *testing.T) {

	timeRef := func(hours int, minutes int) *metav1.Time {
		t := metav1.Date(2021, 2, 1, hours, minutes, 0, 0, time.UTC)
		return &t
	}
	tests := []struct {
		name          string
		app           App
		now           metav1.Time
		wantNoChanges bool
		wantApp       App
		wantErr       string
	}{
		{
			name: "happy path - do canary",
			now:  *timeRef(10, 31),
			app: App{
				Spec: AppSpec{
					Canary: CanarySpec{
						Steps:             3,
						StepWeight:        33,
						StepTimeInteval:   10 * time.Minute,
						NextScheduledTime: timeRef(10, 30),
						CurrentStep:       1,
						Active:            true,
					},
					Deployments: []AppDeploymentSpec{
						{Version: 2, RoutingSettings: RoutingSettings{Weight: 67}},
						{Version: 3, RoutingSettings: RoutingSettings{Weight: 33}},
					},
				},
			},
			wantApp: App{
				Spec: AppSpec{
					Canary: CanarySpec{
						Steps:             3,
						StepWeight:        33,
						StepTimeInteval:   10 * time.Minute,
						NextScheduledTime: timeRef(10, 40),
						CurrentStep:       2,
						Active:            true,
					},
					Deployments: []AppDeploymentSpec{
						{Version: 2, RoutingSettings: RoutingSettings{Weight: 34}},
						{Version: 3, RoutingSettings: RoutingSettings{Weight: 66}},
					},
				},
			},
		},
		{
			name: "happy path - the last step of canary",
			now:  *timeRef(10, 31),
			app: App{
				Spec: AppSpec{
					Canary: CanarySpec{
						Steps:             3,
						StepWeight:        33,
						StepTimeInteval:   10 * time.Minute,
						NextScheduledTime: timeRef(10, 30),
						CurrentStep:       2,
						Active:            true,
					},
					Deployments: []AppDeploymentSpec{
						{Version: 2, RoutingSettings: RoutingSettings{Weight: 34}},
						{Version: 3, RoutingSettings: RoutingSettings{Weight: 66}},
					},
				},
			},
			wantApp: App{
				Spec: AppSpec{
					Canary: CanarySpec{
						Steps:           3,
						StepWeight:      33,
						StepTimeInteval: 10 * time.Minute,
						CurrentStep:     3,
						Active:          false,
					},
					Deployments: []AppDeploymentSpec{
						{Version: 3, RoutingSettings: RoutingSettings{Weight: 100}},
					},
				},
			},
		},
		{
			name:          "canary is not active - no changes",
			now:           *timeRef(10, 31),
			wantNoChanges: true,
			app: App{
				Spec: AppSpec{
					Canary: CanarySpec{
						Steps:             3,
						StepWeight:        33,
						StepTimeInteval:   10 * time.Minute,
						NextScheduledTime: timeRef(10, 30),
						CurrentStep:       2,
						Active:            false,
					},
					Deployments: []AppDeploymentSpec{
						{Version: 2, RoutingSettings: RoutingSettings{Weight: 34}},
						{Version: 3, RoutingSettings: RoutingSettings{Weight: 66}},
					},
				},
			},
		},
		{
			name:          "too early to do canary - no changes",
			now:           *timeRef(10, 25),
			wantNoChanges: true,
			app: App{
				Spec: AppSpec{
					Canary: CanarySpec{
						Steps:             3,
						StepWeight:        33,
						StepTimeInteval:   10 * time.Minute,
						NextScheduledTime: timeRef(10, 30),
						CurrentStep:       2,
						Active:            false,
					},
					Deployments: []AppDeploymentSpec{
						{Version: 2, RoutingSettings: RoutingSettings{Weight: 34}},
						{Version: 3, RoutingSettings: RoutingSettings{Weight: 66}},
					},
				},
			},
		},
		{
			name:          "error - nextScheduledTime is not set",
			now:           *timeRef(10, 45),
			wantNoChanges: true,
			wantErr:       "canary is active but the next step is not scheduled",
			app: App{
				Spec: AppSpec{
					Canary: CanarySpec{
						Steps:           3,
						StepWeight:      33,
						StepTimeInteval: 10 * time.Minute,
						CurrentStep:     2,
						Active:          true,
					},
					Deployments: []AppDeploymentSpec{
						{Version: 2, RoutingSettings: RoutingSettings{Weight: 34}},
						{Version: 3, RoutingSettings: RoutingSettings{Weight: 66}},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.app.DoCanary(tt.now)
			originalApp := *tt.app.DeepCopy()
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Equal(t, err.Error(), tt.wantErr)
				return
			}
			require.Nil(t, err)

			if tt.wantNoChanges {
				require.Equal(t, originalApp, tt.app)
				return
			}
			require.Equal(t, tt.wantApp, tt.app)
		})
	}
}
