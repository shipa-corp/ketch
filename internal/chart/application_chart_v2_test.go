package chart

import (
	"testing"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNewAppChart(t *testing.T) {
	app := &ketchv1.Application{
		Spec: ketchv1.ApplicationSpec{
			Components: []ketchv1.ComponentLink{{
				Name: "Frontend",
				Type: "webserver",
				Properties: map[string]runtime.RawExtension{
					"image": runtime.RawExtension{Raw: []byte("image: me-my-frontend:1.2.3")},
				},
			}},
		},
	}
	app.SetName("test-app")

	tests := []struct {
		name              string
		application       *ketchv1.Application
		components        map[ketchv1.ComponentType]ketchv1.ComponentSpec
		wantYamlsFilename string
		wantErr           bool
	}{
		{
			name:        "success",
			application: app,
			components: map[ketchv1.ComponentType]ketchv1.ComponentSpec{
				"webserver": ketchv1.ComponentSpec{
					Schematic: ketchv1.Schematic{
						Kube: &ketchv1.Kube{
							Templates: []ketchv1.KubeTemplate{{
								Template: runtime.RawExtension{Raw: []byte(`
apiVersion: apps/v1
kind: Deployment
spec:
  selector:
    matchLabels:
      app.ketch.io/component: frontend
  template:
    metadata:
      labels:
        app.ketch.io/component: frontend
    spec:
      containers:
      - name: frontend
        image: some/image:latest
        ports:
        - containerPort: 80
        livenessProbe:
          httpGet:
            path: /
            port: 80
          readinessProbe:
          httpGet:
            path: /
            port: 80
`)},
								Parameters: []ketchv1.Parameter{{
									Name:     "image",
									Required: true,
									Type:     "string",
									FieldPaths: []string{
										"spec.template.spec.containers[0].image",
									},
								}},
							}},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewApplicationChart(tt.application, tt.components)
			if tt.wantErr {
				require.Nil(t, err, "New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.Nil(t, err)

			t.Log(got)

			//chartConfig := ChartConfig{
			//	Version: "0.0.1",
			//	AppName: tt.application.Name,
			//}
			//client := HelmClient{cfg: &action.Configuration{KubeClient: &fake.PrintingKubeClient{}, Releases: storage.Init(driver.NewMemory())}, namespace: "mock-namespace"}
			//release, err := client.UpdateChart(*got, chartConfig, func(install *action.Install) {
			//	install.DryRun = true
			//	install.ClientOnly = true
			//})
			//require.Nil(t, err)
			//
			//fmt.Println(release)
		})
	}
}

func TestRenderComponentTemplates(t *testing.T) {
	componentSpec := &ketchv1.ComponentSpec{
		Schematic: ketchv1.Schematic{
			Kube: &ketchv1.Kube{
				Templates: []ketchv1.KubeTemplate{{
					Template: runtime.RawExtension{Raw: []byte(`
apiVersion: apps/v1
kind: Deployment
spec:
  selector:
    matchLabels:
      app.ketch.io/component: frontend
  template:
    metadata:
      labels:
        app.ketch.io/component: frontend
    spec:
      containers:
        - name: frontend
          image: some/image:latest
          ports:
            - containerPort: 80
          livenessProbe:
            httpGet:
              path: /
              port: 80
            readinessProbe:
              httpGet:
                path: /
                port: 80
`)},
					Parameters: []ketchv1.Parameter{{
						Name:     "image",
						Required: true,
						Type:     "string",
						FieldPaths: []string{
							"spec.template.spec.containers[0].image",
						},
					}},
				}},
			},
		},
	}
	tests := []struct {
		componentSpec *ketchv1.ComponentSpec
		componentLink *ketchv1.ComponentLink
		details       string
		err           error
	}{
		{
			componentSpec: componentSpec,
			componentLink: &ketchv1.ComponentLink{},
			details:       "success",
		},
	}
	for _, test := range tests {
		res, err := RenderComponentTemplates(test.componentSpec, "test-component")
		require.Nil(t, err)
		t.Log(res)
	}
}
