package main

import (
	"bytes"
	"context"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/api/errors"
	"os"
	"path"
	"testing"

	registryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/build"
	"github.com/shipa-corp/ketch/internal/deploy"
	"github.com/shipa-corp/ketch/internal/pack"
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

type packMocker struct{}

func (packMocker) BuildAndPushImage(ctx context.Context, req pack.BuildRequest) error {
	return nil
}

func getImageConfig(ctx context.Context, args deploy.ImageConfigRequest) (*registryv1.ConfigFile, error) {
	return &registryv1.ConfigFile{
		Config: registryv1.Config{
			Cmd: []string{"/bin/eatme"},
		},
	}, nil
}

var (
	ketchYaml string = `
kubernetes:
  processes:
    web:
      ports:
        - name: apache-http # an optional name for the port
          protocol: TCP
          port: 80 # The port that is going to be exposed on the router.
          target_port: 9999 # The port on which the application listens on.
    worker:
      ports:
        - name: http
          protocol: TCP
          port: 80
    worker-2:
      ports: []
`
	procfile string = `
web: python app.py
worker: python app.py
`
	packBuildMetadata string = "{\"bom\":null,\"buildpacks\":[{\"id\":\"heroku/python\",\"version\":\"0.3.1\"},{\"id\":\"heroku/procfile\",\"version\":\"0.6.2\"}],\"launcher\":{\"version\":\"0.11.3\",\"source\":{\"git\":{\"repository\":\"github.com/buildpacks/lifecycle\",\"commit\":\"aa4bbac\"}}},\"processes\":[{\"type\":\"web\",\"command\":\"python app.py\",\"args\":null,\"direct\":false,\"buildpackID\":\"heroku/procfile\"},{\"type\":\"worker\",\"command\":\"python app.py\",\"args\":null,\"direct\":false,\"buildpackID\":\"heroku/procfile\"},{\"type\":\"worker1\",\"command\":\"python app.py\",\"args\":null,\"direct\":false,\"buildpackID\":\"heroku/procfile\"}]}"
)

func TestNewCommand(t *testing.T) {
	tt := []struct {
		name        string
		params      *deploy.Services
		arguments   []string
		setup       func(t *testing.T)
		userDefault string
		validate    func(t *testing.T, m *mockClient)
		wantError   bool
	}{
		{
			name: "change builder from previous deploy",
			arguments: []string{
				"myapp",
				"src",
				"--image", "shipa/go-sample:latest",
				"--builder", "some other builder",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
				require.Nil(t, ioutil.WriteFile("src/Procfile", []byte(procfile), 0600))
			},
			validate: func(t *testing.T, mock *mockClient) {
				require.Equal(t, "some other builder", mock.app.Spec.Builder)
			},
			params: &deploy.Services{
				Client: func() *mockClient {
					m := newMockClient()
					m.app.Spec.Builder = "superduper builder"
					return m
				}(),
				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		{
			name: "use builder from previous deploy",
			arguments: []string{
				"myapp",
				"src",
				"--framework", "initialframework",
				"--image", "shipa/go-sample:latest",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
				require.Nil(t, ioutil.WriteFile("src/Procfile", []byte(procfile), 0600))
			},
			validate: func(t *testing.T, mock *mockClient) {
				require.Equal(t, "superduper builder", mock.app.Spec.Builder)
			},
			params: &deploy.Services{
				Client: func() *mockClient {
					m := newMockClient()
					m.app.Spec.Builder = "superduper builder"
					return m
				}(),
				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		{
			name: "use default builder for new app",
			arguments: []string{
				"myapp",
				"src",
				"--framework", "myframework",
				"--image", "shipa/go-sample:latest",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
				require.Nil(t, ioutil.WriteFile("src/Procfile", []byte(procfile), 0600))
			},
			validate: func(t *testing.T, mock *mockClient) {
				require.Equal(t, deploy.DefaultBuilder, mock.app.Spec.Builder)
			},
			params: &deploy.Services{
				Client: func() *mockClient {
					m := newMockClient()
					m.get[1] = func(_ *mockClient, _ runtime.Object) error {
						return errors.NewNotFound(v1.Resource(""), "")
					}
					return m
				}(),
				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		{
			name: "use user default builder for new app",
			arguments: []string{
				"myapp",
				"src",
				"--framework", "myframework",
				"--image", "shipa/go-sample:latest",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
				require.Nil(t, ioutil.WriteFile("src/Procfile", []byte(procfile), 0600))
			},
			userDefault: "newDefault",
			validate: func(t *testing.T, mock *mockClient) {
				require.Equal(t, "newDefault", mock.app.Spec.Builder)
			},
			params: &deploy.Services{
				Client: func() *mockClient {
					m := newMockClient()
					m.get[1] = func(_ *mockClient, _ runtime.Object) error {
						return errors.NewNotFound(v1.Resource(""), "")
					}
					return m
				}(),
				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		{
			name: "don't update builder on previous deployment",
			arguments: []string{
				"myapp",
				"src",
				"--image", "shipa/go-sample:latest",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
				require.Nil(t, ioutil.WriteFile("src/Procfile", []byte(procfile), 0600))
			},
			userDefault: "newDefault",
			validate: func(t *testing.T, mock *mockClient) {
				require.Equal(t, mock.app.Spec.Builder, "initialBuilder")

			},
			params: &deploy.Services{
				Client: func() *mockClient {
					m := newMockClient()
					m.app.Spec.Builder = "initialBuilder"
					m.app.Spec.Deployments = []ketchv1.AppDeploymentSpec{
						{
							Image:           "shipa/go-sample:latest",
							Version:         1,
							Processes:       nil,
							KetchYaml:       nil,
							Labels:          nil,
							RoutingSettings: ketchv1.RoutingSettings{},
							ExposedPorts:    nil,
						},
					}
					return m
				}(),

				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		{
			name: "use assigned builder for new app",
			arguments: []string{
				"myapp",
				"src",
				"--framework", "myframework",
				"--image", "shipa/go-sample:latest",
				"--builder", "superduper",
				"--build-packs", "pack1,pack2",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
				require.Nil(t, ioutil.WriteFile("src/Procfile", []byte(procfile), 0600))
			},
			validate: func(t *testing.T, mock *mockClient) {
				require.Equal(t, "superduper", mock.app.Spec.Builder)
				require.Equal(t, []string{"pack1", "pack2"}, mock.app.Spec.BuildPacks)
			},
			params: &deploy.Services{
				Client: func() *mockClient {
					m := newMockClient()
					m.get[1] = func(_ *mockClient, _ runtime.Object) error {
						return errors.NewNotFound(v1.Resource(""), "")
					}
					return m
				}(),
				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		// build from source, creates app
		{
			name: "happy path build from source",
			arguments: []string{
				"myapp",
				"src",
				"--framework", "myframework",
				"--image", "shipa/go-sample:latest",
				"--env", "foo=bar,zip=zap",
				"--builder", "newbuilder",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
				require.Nil(t, ioutil.WriteFile("src/ketch.yaml", []byte(ketchYaml), 0600))
				require.Nil(t, ioutil.WriteFile("src/Procfile", []byte(procfile), 0600))
			},
			validate: func(t *testing.T, mock *mockClient) {
				require.Equal(t, "myframework", mock.app.Spec.Framework)
				require.Equal(t, "newbuilder", mock.app.Spec.Builder)
				require.Len(t, mock.app.Spec.Deployments, 1)
				require.Len(t, mock.app.Spec.Deployments[0].KetchYaml.Kubernetes.Processes, 3)
				require.Len(t, mock.app.Spec.Env, 2)
			},
			params: &deploy.Services{
				Client: func() *mockClient {
					m := newMockClient()
					m.get[1] = func(_ *mockClient, _ runtime.Object) error {
						return errors.NewNotFound(v1.Resource(""), "")
					}
					return m
				}(),
				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		// build from source, updates app
		{
			name: "with custom yaml path",
			arguments: []string{
				"myapp",
				"src",
				"--image", "shipa/go-sample:latest",
				"--env", "foo=bar,zip=zap",
				"--ketch-yaml", "config/ketch.yaml",
				"--registry-secret", "supersecret",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()

				require.Nil(t, os.Mkdir(path.Join(dir, "config"), 0700))
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.MkdirAll(path.Join(dir, "src/include1"), 0700))
				require.Nil(t, os.MkdirAll(path.Join(dir, "src/include2"), 0700))
				require.Nil(t, os.Chdir(dir))
				require.Nil(t, ioutil.WriteFile("config/ketch.yaml", []byte(ketchYaml), 0600))
				require.Nil(t, ioutil.WriteFile("src/Procfile", []byte(procfile), 0600))
			},
			validate: func(t *testing.T, mock *mockClient) {
				require.Len(t, mock.app.Spec.Deployments, 1)
				require.Len(t, mock.app.Spec.Deployments[0].KetchYaml.Kubernetes.Processes, 3)
				require.Len(t, mock.app.Spec.Env, 2)
				require.Equal(t, "supersecret", mock.app.Spec.DockerRegistry.SecretName)
			},
			params: &deploy.Services{
				Client: func() *mockClient {
					m := newMockClient()

					return m
				}(),
				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		{
			name: "happy path with canary deploy build from source",
			arguments: []string{
				"myapp",
				"src",
				"--steps", "4",
				"--step-interval", "1h",
				"--image", "shipa/go-sample:latest",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
				require.Nil(t, ioutil.WriteFile("src/Procfile", []byte(procfile), 0600))
			},
			validate: func(t *testing.T, mock *mockClient) {
				require.Equal(t, mock.app.Spec.Framework, "initialframework")

			},
			params: &deploy.Services{
				Client: func() *mockClient {
					m := newMockClient()
					m.app.Spec.Deployments = []ketchv1.AppDeploymentSpec{
						{
							Image:           "shipa/go-sample:latest",
							Version:         1,
							Processes:       nil,
							KetchYaml:       nil,
							Labels:          nil,
							RoutingSettings: ketchv1.RoutingSettings{},
							ExposedPorts:    nil,
						},
					}
					return m
				}(),

				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		{
			name: "happy path build from image",
			arguments: []string{
				"myapp",
				"--framework", "myframework",
				"--image", "shipa/go-sample:latest",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
			},
			params: &deploy.Services{
				Client: func() *mockClient {
					m := newMockClient()
					m.get[1] = func(_ *mockClient, _ runtime.Object) error {
						return errors.NewNotFound(v1.Resource(""), "")
					}
					return m
				}(),

				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		{
			name:      "missing source path",
			wantError: true,
			arguments: []string{
				"myapp",
				"src",
				"--framework", "myframework",
				"--image", "shipa/go-sample:latest",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Chdir(dir))
			},
			params: &deploy.Services{
				Client: func() *mockClient {
					m := newMockClient()
					m.get[1] = func(_ *mockClient, _ runtime.Object) error {
						return errors.NewNotFound(v1.Resource(""), "")
					}
					return m
				}(),

				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		{
			name:      "procfile flag with source specified",
			wantError: true,
			arguments: []string{
				"myapp",
				"src",
				"--framework", "myframework",
				"--image", "shipa/go-sample:latest",
				"--procfile", "./Procfile",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
			},
			validate: func(t *testing.T, mock *mockClient) {
				require.Equal(t, "myframework", mock.app.Spec.Framework)
			},
			params: &deploy.Services{
				Client: func() *mockClient {
					m := newMockClient()
					m.get[1] = func(_ *mockClient, _ runtime.Object) error {
						return errors.NewNotFound(v1.Resource(""), "")
					}
					return m
				}(),
				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		{
			name: "with environment variables",
			arguments: []string{
				"myapp",
				"src",
				"--framework", "myframework",
				"--image", "shipa/go-sample:latest",
				"--env", "foo=bar,bobb=dobbs",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
				require.Nil(t, ioutil.WriteFile("src/Procfile", []byte(procfile), 0600))
			},
			params: &deploy.Services{
				Client: func() *mockClient {
					m := newMockClient()
					m.get[1] = func(_ *mockClient, _ runtime.Object) error {
						return errors.NewNotFound(v1.Resource(""), "")
					}
					return m
				}(),

				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		{
			name:      "with messed up environment variables",
			wantError: true,
			arguments: []string{
				"myapp",
				"src",
				"--framework", "myframework",
				"--image", "shipa/go-sample:latest",
				"--env", "foo=bar,bobbdobbs",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
				require.Nil(t, ioutil.WriteFile("src/Procfile", []byte(procfile), 0600))
			},
			params: &deploy.Services{
				Client: func() *mockClient {
					m := newMockClient()
					m.get[1] = func(_ *mockClient, _ runtime.Object) error {
						return errors.NewNotFound(v1.Resource(""), "")
					}
					return m
				}(),
				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		{
			name:      "missing Procfile in src",
			wantError: true,
			arguments: []string{
				"myapp",
				"src",
				"--framework", "myframework",
				"--image", "shipa/go-sample:latest",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
			},
			params: &deploy.Services{
				Client: func() *mockClient {
					m := newMockClient()
					m.get[1] = func(_ *mockClient, _ runtime.Object) error {
						return errors.NewNotFound(v1.Resource(""), "")
					}
					return m
				}(),
				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		{
			name:      "illicit use of --unit-version without --units",
			wantError: true,
			arguments: []string{
				"myapp",
				"src",
				"--framework", "myframework",
				"--image", "shipa/go-sample:latest",
				"--unit-version", "7",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
				require.Nil(t, ioutil.WriteFile("src/Procfile", []byte(procfile), 0600))
			},
			params: &deploy.Services{
				Client: func() *mockClient {
					m := newMockClient()
					m.get[1] = func(_ *mockClient, _ runtime.Object) error {
						return errors.NewNotFound(v1.Resource(""), "")
					}
					return m
				}(),
				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		{
			name:      "illicit use of --unit-process without --units",
			wantError: true,
			arguments: []string{
				"myapp",
				"src",
				"--framework", "myframework",
				"--image", "shipa/go-sample:latest",
				"--unit-process", "test",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
				require.Nil(t, ioutil.WriteFile("src/Procfile", []byte(procfile), 0600))
			},
			params: &deploy.Services{
				Client: func() *mockClient {
					m := newMockClient()
					m.get[1] = func(_ *mockClient, _ runtime.Object) error {
						return errors.NewNotFound(v1.Resource(""), "")
					}
					return m
				}(),
				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		{
			name: "happy path with --units build from source, update deployment spec",
			arguments: []string{
				"myapp",
				"src",
				"--units", "4",
				"--image", "shipa/go-sample:latest",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
				require.Nil(t, ioutil.WriteFile("src/Procfile", []byte(procfile), 0600))
			},
			validate: func(t *testing.T, mock *mockClient) {
				// changes from the previous deployment defined below to the one created by procfile variable above
				require.Equal(t, mock.app.Spec.Deployments[0].Processes[1].Name, "worker")
				require.Equal(t, mock.app.Spec.Deployments[0].Processes[1].Cmd[0], "worker")
				for _, process := range mock.app.Spec.Deployments[0].Processes {
					require.Equal(t, *process.Units, 4)
				}
				require.Equal(t, mock.app.Spec.Framework, "initialframework")

			},
			params: &deploy.Services{
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
								{
									Name: "worker2",
									Cmd:  []string{"do", "work"},
								},
							},
							KetchYaml:       nil,
							Labels:          nil,
							RoutingSettings: ketchv1.RoutingSettings{},
							ExposedPorts:    nil,
						},
					}
					return m
				}(),

				KubeClient: fake.NewSimpleClientset(),
				Builder:    build.GetSourceHandler(&packMocker{}),

				GetImageConfig: func(ctx context.Context, args deploy.ImageConfigRequest) (*registryv1.ConfigFile, error) {
					return &registryv1.ConfigFile{
						Config: registryv1.Config{
							Cmd:    []string{"/bin/eatme"},
							Labels: map[string]string{"io.buildpacks.build.metadata": packBuildMetadata},
						},
					}, nil
				},
				Wait:   nil,
				Writer: &bytes.Buffer{},
			},
		},
		{
			name: "happy path with --units and --unit-process build from source, update deployment spec",
			arguments: []string{
				"myapp",
				"src",
				"--units", "4",
				"--unit-process", "worker",
				"--image", "shipa/go-sample:latest",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
				require.Nil(t, ioutil.WriteFile("src/Procfile", []byte(procfile), 0600))
			},
			validate: func(t *testing.T, mock *mockClient) {
				require.Nil(t, mock.app.Spec.Deployments[0].Processes[0].Units)
				require.Equal(t, *mock.app.Spec.Deployments[0].Processes[1].Units, 4)
				// changes from the previous deployment defined below to the one created by procfile variable above
				require.Equal(t, mock.app.Spec.Deployments[0].Processes[1].Name, "worker")
				require.Equal(t, mock.app.Spec.Deployments[0].Processes[1].Cmd[0], "worker")
				require.Equal(t, mock.app.Spec.Framework, "initialframework")

			},
			params: &deploy.Services{
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
							KetchYaml:       nil,
							Labels:          nil,
							RoutingSettings: ketchv1.RoutingSettings{},
							ExposedPorts:    nil,
						},
					}
					return m
				}(),

				KubeClient: fake.NewSimpleClientset(),
				Builder:    build.GetSourceHandler(&packMocker{}),
				GetImageConfig: func(ctx context.Context, args deploy.ImageConfigRequest) (*registryv1.ConfigFile, error) {
					return &registryv1.ConfigFile{
						Config: registryv1.Config{
							Cmd:    []string{"/bin/eatme"},
							Labels: map[string]string{"io.buildpacks.build.metadata": packBuildMetadata},
						},
					}, nil
				},
				Wait:   nil,
				Writer: &bytes.Buffer{},
			},
		},
		{
			name: "happy path with --units, --unit-process, and --unit-version build from image",
			arguments: []string{
				"myapp",
				"--units", "4",
				"--unit-process", "worker1",
				"--unit-version", "1",
				"--image", "shipa/go-sample:latest",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
			},
			validate: func(t *testing.T, mock *mockClient) {
				require.Nil(t, mock.app.Spec.Deployments[0].Processes[0].Units)
				require.Equal(t, *mock.app.Spec.Deployments[0].Processes[1].Units, 4)
				require.Nil(t, mock.app.Spec.Deployments[1].Processes[0].Units)
				require.Nil(t, mock.app.Spec.Deployments[1].Processes[1].Units)
				require.Equal(t, mock.app.Spec.Framework, "initialframework")

			},
			params: &deploy.Services{
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
							KetchYaml:       nil,
							Labels:          nil,
							RoutingSettings: ketchv1.RoutingSettings{},
							ExposedPorts:    nil,
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
							KetchYaml:       nil,
							Labels:          nil,
							RoutingSettings: ketchv1.RoutingSettings{},
							ExposedPorts:    nil,
						},
					}
					return m
				}(),

				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// restore working dir so we don't screw up other tests
			wd, err := os.Getwd()
			require.Nil(t, err)
			defer func() {
				_ = os.Chdir(wd)
			}()

			if tc.setup != nil {
				tc.setup(t)
			}
			cmd := newAppDeployCmd(nil, tc.params, tc.userDefault)
			cmd.SetArgs(tc.arguments)
			err = cmd.Execute()
			if tc.wantError {
				t.Logf("got error %s", err)
				require.NotNil(t, err)
				return
			}

			require.Nil(t, err)
			if tc.validate != nil {
				mock, ok := tc.params.Client.(*mockClient)
				require.True(t, ok)
				tc.validate(t, mock)
			}
		})
	}
}
