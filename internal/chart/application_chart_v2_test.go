package chart

import (
	"log"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

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
					"image": runtime.RawExtension{Raw: []byte("me-my-frontend:1.2.3")},
				},
			}},
		},
	}
	app.SetName("test-app")

	expectedDeployment := `apiVersion: apps/v1
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
        image: me-my-frontend:1.2.3
        ports:
        - containerPort: 80
        livenessProbe:
          httpGet:
            path: /
            port: 80
        readinessProbe:
          httpGet:
            path: /
            port: 80`

	tests := []struct {
		name        string
		application *ketchv1.Application
		components  map[ketchv1.ComponentType]ketchv1.ComponentSpec
		expected    *ApplicationChartV2
		wantErr     bool
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
			expected: &ApplicationChartV2{
				Name:      "test-app",
				Templates: map[string]string{"Frontend": expectedDeployment},
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

			for name, template := range tt.expected.Templates {
				var expectedMap map[string]interface{}
				err = yaml.Unmarshal([]byte(template), &expectedMap)
				require.Nil(t, err)

				var receivedMap map[string]interface{}
				err = yaml.Unmarshal([]byte(got.Templates[name]), &receivedMap)
				require.Nil(t, err)
			}
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
metadata:
  name: deployment
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
						Name:     "name",
						Required: true,
						Type:     "string",
						FieldPaths: []string{
							"metadata.name",
							"metadata.labels.app",
							"spec.selector.matchLabels['app.ketch.io/component']",
							"spec.template.metadata.labels['app.ketch.io/component']",
						},
					}, {
						Name:     "image",
						Required: true,
						Type:     "string",
						FieldPaths: []string{
							"spec.template.spec.containers[0].image",
						},
					}, {
						Name:     "port",
						Required: false,
						Type:     "number",
						FieldPaths: []string{
							"spec.template.spec.containers[0].ports[0].containerPort",
							"spec.template.spec.containers[0].livenessProbe.httpGet.port",
							"spec.template.spec.containers[0].readinessProbe.httpGet.port",
						},
					}},
				}},
			},
		},
	}
	tests := []struct {
		componentSpec     *ketchv1.ComponentSpec
		componentLink     *ketchv1.ComponentLink
		expectedTemplates map[string]string
		details           string
		err               error
	}{
		{
			componentSpec: componentSpec,
			componentLink: &ketchv1.ComponentLink{
				Name: "component-implementation",
				Type: "frontend-component",
				Properties: map[string]runtime.RawExtension{
					"name": runtime.RawExtension{
						Raw: []byte("hello-world"),
					},
					"image": runtime.RawExtension{
						Raw: []byte("me/my-frontend:1.2.3"),
					},
					"port": runtime.RawExtension{
						Raw: []byte("9999"),
					},
				},
			},
			expectedTemplates: map[string]string{"component-implementation": `apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: hello-world
  name: hello-world
spec:
  selector:
    matchLabels:
      app.ketch.io/component: hello-world
  template:
    metadata:
      labels:
        app.ketch.io/component: hello-world
    spec:
      containers:
        - image: me/my-frontend:1.2.3
          livenessProbe:
            httpGet:
              path: /
              port: 9999
          name: frontend
          ports:
            - containerPort: 9999
          readinessProbe:
            httpGet:
              path: /
              port: 9999`},
			details: "success",
		},
	}
	for _, test := range tests {
		templates, err := RenderComponentTemplates(test.componentSpec, test.componentLink)
		require.Nil(t, err)

		for name, template := range test.expectedTemplates {
			var expectedMap map[string]interface{}
			err = yaml.Unmarshal([]byte(template), &expectedMap)
			require.Nil(t, err)

			var receivedMap map[string]interface{}
			err = yaml.Unmarshal([]byte(templates[name]), &receivedMap)
			require.Nil(t, err)

			require.Equal(t, expectedMap, receivedMap)
		}
	}
}

func TestSetParameterValuesToKubeObject(t *testing.T) {
	componentSpec := []byte(`apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: frontend
          image: some/image:latest
          decimal: 1.1
          required: false
`)

	tests := []struct {
		expectedSpec      []byte
		paramValueSetting paramValueSetting
		expectedError     error
	}{
		{
			expectedSpec: []byte(`apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: frontend
          image: TEST/IMAGE
          decimal: 1.1
          required: false
`),
			paramValueSetting: paramValueSetting{
				ValueType:  "string",
				Value:      runtime.RawExtension{Raw: []byte("TEST/IMAGE")},
				FieldPaths: []string{"spec.template.spec.containers[0].image"},
			},
		},
		{
			expectedSpec: []byte(`apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: frontend
          image: some/image:latest
          decimal: 1.1
          required: true
`),
			paramValueSetting: paramValueSetting{
				ValueType:  "bool",
				Value:      runtime.RawExtension{Raw: []byte("true")},
				FieldPaths: []string{"spec.template.spec.containers[0].required"},
			},
		},
		{
			expectedSpec: []byte(`apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: frontend
          image: some/image:latest
          decimal: 2.1
          required: false
`),
			paramValueSetting: paramValueSetting{
				ValueType:  "number",
				Value:      runtime.RawExtension{Raw: []byte("2.1")},
				FieldPaths: []string{"spec.template.spec.containers[0].decimal"},
			},
		},
	}

	for _, test := range tests {
		// expected unstructured.Unstructured object
		var expectedSpecMap map[string]interface{}
		err := yaml.Unmarshal(test.expectedSpec, &expectedSpecMap)
		if err != nil {
			log.Fatal(err)
		}
		expected := unstructured.Unstructured{
			Object: expectedSpecMap,
		}
		var specMap map[string]interface{}
		err = yaml.Unmarshal(componentSpec, &specMap)
		if err != nil {
			log.Fatal(err)
		}

		// set parameters on test unstructured.Unstructured object
		raw := unstructured.Unstructured{Object: specMap}
		vals := []paramValueSetting{test.paramValueSetting}
		err = setParameterValuesToKubeObj(&raw, vals)

		// assertions
		require.Nil(t, err)
		require.Equal(t, expected.Object, raw.Object)
	}
}
