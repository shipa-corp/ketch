package deploy

import (
	"context"
	registryv1 "github.com/google/go-containerregistry/pkg/v1"
	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/chart"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"

	"github.com/stretchr/testify/require"
)

type getterCreatorMockFn func(m *mockClient, obj runtime.Object) error
type funcMap map[int]getterCreatorMockFn

type mockClient struct {
	get    funcMap
	create funcMap
	update funcMap

	app       *ketchv1.App
	framework *ketchv1.Framework

	getCounter    int
	createCounter int
	updateCounter int
}

func newMockClient() *mockClient {
	return &mockClient{
		get:    make(funcMap),
		update: make(funcMap),
		create: make(funcMap),
		app: &ketchv1.App{
			Spec: ketchv1.AppSpec{
				Description: "foo",
				Framework:   "initialframework",
				Builder:     "initialbuilder",
			},
		},
		framework: &ketchv1.Framework{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       ketchv1.FrameworkSpec{},
			Status:     ketchv1.FrameworkStatus{},
		},
	}
}

func (m *mockClient) Get(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
	m.getCounter++

	if f, ok := m.get[m.getCounter]; ok {
		return f(m, obj)
	}

	switch v := obj.(type) {
	case *ketchv1.App:
		*v = *m.app
		return nil
	case *ketchv1.Framework:
		*v = *m.framework
		return nil
	}
	panic("unhandled type")
}

func (m *mockClient) Create(_ context.Context, obj runtime.Object, _ ...client.CreateOption) error {
	m.createCounter++

	if f, ok := m.create[m.createCounter]; ok {
		return f(m, obj)
	}

	switch v := obj.(type) {
	case *ketchv1.App:
		m.app = v
		return nil
	case *ketchv1.Framework:
		m.framework = v
		return nil
	}
	panic("unhandled type")
}

func (m *mockClient) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	m.updateCounter++

	if f, ok := m.update[m.updateCounter]; ok {
		return f(m, obj)
	}

	switch v := obj.(type) {
	case *ketchv1.App:
		m.app = v
		return nil
	case *ketchv1.Framework:
		m.framework = v
		return nil
	}
	panic("unhandled type")
}

func Test_updateAppCRD(t *testing.T) {
	type args struct {
		ctx     context.Context
		svc     *Services
		appName string
		args    updateAppCRDRequest
	}
	tests := []struct {
		name     string
		args     args
		want     *ketchv1.App
		validate func(t *testing.T, m *mockClient)
		wantErr  bool
	}{
		{
			name: "multiple deployments, canary is false",
			args: args{
				ctx:     context.Background(),
				appName: "test-app",
				svc: &Services{
					Client: func() *mockClient {
						m := newMockClient()
						m.app.Spec.Deployments = []ketchv1.AppDeploymentSpec{
							{
								Image:   "shipa/go-sample:latest",
								Version: 1,
								Processes: []ketchv1.ProcessSpec{
									{
										Name: "web",
										Cmd:  []string{"/cnb/process/web"},
									},
									{
										Name: "worker1",
										Cmd:  []string{"do", "work"},
									},
								},
							},
							{
								Image:   "shipa/go-sample:latest",
								Version: 2,
								Processes: []ketchv1.ProcessSpec{
									{
										Name: "web",
										Cmd:  []string{"/cnb/process/web"},
									},
									{
										Name: "worker1",
										Cmd:  []string{"do", "work"},
									},
								},
							},
						}
						return m
					}(),
				},
			},
			wantErr: true,
		},
		{
			name: "multiple deployments, canary is true, and return updated units",
			args: args{
				ctx:     context.Background(),
				appName: "test-app",
				args: updateAppCRDRequest{
					units:   3,
					version: 2,
					process: "worker1",
				},
				svc: &Services{
					Client: func() *mockClient {
						m := newMockClient()
						// must be canary to have more than one deployment
						m.app.Spec.Canary.Active = true
						m.app.Spec.Deployments = []ketchv1.AppDeploymentSpec{
							{
								Image:   "shipa/go-sample:latest",
								Version: 1,
								Processes: []ketchv1.ProcessSpec{
									{
										Name: "web",
										Cmd:  []string{"/cnb/process/web"},
									},
									{
										Name: "worker1",
										Cmd:  []string{"do", "work"},
									},
								},
							},
							{
								Image:   "shipa/go-sample:latest",
								Version: 2,
								Processes: []ketchv1.ProcessSpec{
									{
										Name: "web",
										Cmd:  []string{"/cnb/process/web"},
									},
									{
										Name: "worker1",
										Cmd:  []string{"do", "work"},
									},
								},
							},
						}
						return m
					}(),
				},
			},
			validate: func(t *testing.T, mock *mockClient) {
				// confirm that units updates by version and process
				require.Equal(t, *mock.app.Spec.Deployments[1].Processes[1].Units, 3)
				// confirm that version doesn't increment
				require.Equal(t, mock.app.Spec.Deployments[1].Version, ketchv1.DeploymentVersion(2))
			},
		},
		{
			name: "previous and new image different, update process and version",
			args: args{
				ctx:     context.Background(),
				appName: "test-app",
				args: updateAppCRDRequest{
					image: "test/pack-test:latest",
					procFile: &chart.Procfile{
						Processes:           map[string][]string{"worker": []string{"worker"}},
						RoutableProcessName: "worker",
					},
					configFile: &registryv1.ConfigFile{
						Config: registryv1.Config{
							ExposedPorts: make(map[string]struct{}),
						},
					},
				},
				svc: &Services{
					Client: func() *mockClient {
						m := newMockClient()
						m.app.Spec.DeploymentsCount = 1
						m.app.Spec.Deployments = []ketchv1.AppDeploymentSpec{
							{
								Image:   "shipa/go-sample:latest",
								Version: 1,
								Processes: []ketchv1.ProcessSpec{
									{
										Name: "web",
										Cmd:  []string{"/cnb/process/web"},
									},
								},
							},
						}
						return m
					}(),
				},
			},
			validate: func(t *testing.T, mock *mockClient) {
				// confirm that units update
				require.Equal(t, mock.app.Spec.Deployments[0].Processes[0].Name, "worker")
				// confirm that version doesn't increment
				require.Equal(t, mock.app.Spec.Deployments[0].Version, ketchv1.DeploymentVersion(2))
			},
		},
		{
			name: "previous and new image same, don't update version",
			args: args{
				ctx:     context.Background(),
				appName: "test-app",
				args: updateAppCRDRequest{
					image: "test/pack-test:latest",
					procFile: &chart.Procfile{
						Processes:           map[string][]string{"worker": []string{"worker"}},
						RoutableProcessName: "worker",
					},
					configFile: &registryv1.ConfigFile{
						Config: registryv1.Config{
							ExposedPorts: make(map[string]struct{}),
						},
					},
				},
				svc: &Services{
					Client: func() *mockClient {
						m := newMockClient()
						m.app.Spec.DeploymentsCount = 1
						m.app.Spec.Deployments = []ketchv1.AppDeploymentSpec{
							{
								Image:   "test/pack-test:latest",
								Version: 1,
								Processes: []ketchv1.ProcessSpec{
									{
										Name: "worker",
										Cmd:  []string{"worker"},
									},
								},
							},
						}
						return m
					}(),
				},
			},
			validate: func(t *testing.T, mock *mockClient) {
				// confirm that units update
				require.Equal(t, mock.app.Spec.Deployments[0].Processes[0].Name, "worker")
				// confirm that version doesn't increment
				require.Equal(t, mock.app.Spec.Deployments[0].Version, ketchv1.DeploymentVersion(1))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := updateAppCRD(tt.args.ctx, tt.args.svc, tt.args.appName, tt.args.args)

			if tt.wantErr {
				t.Logf("got error %s", err)
				require.NotNil(t, err)
				return
			}

			require.Nil(t, err)
			if tt.validate != nil {
				mock, ok := tt.args.svc.Client.(*mockClient)
				require.True(t, ok)
				tt.validate(t, mock)
			}
		})
	}
}

func Test_makeProcfile(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *registryv1.ConfigFile
		want    *chart.Procfile
		wantErr bool
	}{
		{
			name: "non-pack image, no entrypoint or commands",
			cfg: &registryv1.ConfigFile{
				Config: registryv1.Config{},
			},
			wantErr: true,
		},
		{
			name: "non-pack image, return procfile",
			cfg: &registryv1.ConfigFile{
				Config: registryv1.Config{
					Entrypoint: []string{"web"},
					Cmd:        []string{"python app.py"},
				},
			},
			want: &chart.Procfile{
				Processes:           map[string][]string{"web": []string{"web", "python app.py"}},
				RoutableProcessName: "web",
			},
		},
		{
			name: "pack image, broken json",
			cfg: &registryv1.ConfigFile{
				Config: registryv1.Config{
					Labels: map[string]string{"io.buildpacks.build.metadata": "{\"processes\": [{\"type\" \"web\"}]}"},
				},
			},
			wantErr: true,
		},
		{
			name: "pack image, no processes",
			cfg: &registryv1.ConfigFile{
				Config: registryv1.Config{
					Labels: map[string]string{"io.buildpacks.build.metadata": "{\"processes\": []}"},
				},
			},
			wantErr: true,
		},
		{
			name: "pack image, returns procfile",
			cfg: &registryv1.ConfigFile{
				Config: registryv1.Config{
					Labels: map[string]string{"io.buildpacks.build.metadata": "{\"processes\": [{\"type\": \"web\"}]}"},
				},
			},
			want: &chart.Procfile{
				Processes:           map[string][]string{"web": []string{"web"}},
				RoutableProcessName: "web",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := makeProcfile(tt.cfg)

			if tt.wantErr {
				t.Logf("got error %s", err)
				require.NotNil(t, err)
				return
			}
			require.Equal(t, tt.want, got)
		})
	}
}
