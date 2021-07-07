package deploy

import (
	"os"
	"testing"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"

	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func TestGetChangeSetFromYaml(t *testing.T) {
	tests := []struct {
		description string
		yaml        string
		options     *Options
		changeSet   *ChangeSet
		errStr      string
	}{
		{
			description: "success",
			yaml: `version: v1
type: Application
name: test
image: gcr.io/kubernetes/sample-app:latest
framework: myframework
description: a test
builder: heroku/buildpacks:20
buildPacks: 
  - test-buildpack
environment:
  - PORT=6666
  - FOO=bar
processes:
  - name: web
    cmd: python app.py
    units: 1
    ports:
      - port: 8888
        targetPort: 6666
        protocol: TCP
    hooks:
      - restart:
          before: pwd
          after: echo "test"
  - name: worker
    cmd: python app.py
    units: 1
    ports:
      - targetPort: 6666
        port: 8888
        protocol: TCP
appUnit: 2
cname:
  dnsName: test.10.10.10.20`,
			changeSet: &ChangeSet{
				appName:              "test",
				appUnit:              intPtr(2),
				yamlStrictDecoding:   true,
				sourcePath:           strPtr("."),
				image:                strPtr("gcr.io/kubernetes/sample-app:latest"),
				description:          strPtr("a test"),
				envs:                 &[]string{"PORT=6666", "FOO=bar"},
				framework:            strPtr("myframework"),
				dockerRegistrySecret: nil,
				builder:              strPtr("heroku/buildpacks:20"),
				buildPacks:           &[]string{"test-buildpack"},
				cname:                &ketchv1.CnameList{"test.10.10.10.20"},
				processes: &[]ketchv1.ProcessSpec{
					{
						Name:  "web",
						Cmd:   []string{"python", "app.py"},
						Units: intPtr(1),
						Env: []ketchv1.Env{
							{
								Name:  "PORT",
								Value: "6666",
							},
							{
								Name:  "FOO",
								Value: "bar",
							},
						},
					},
					{
						Name:  "worker",
						Cmd:   []string{"python", "app.py"},
						Units: intPtr(1),
						Env: []ketchv1.Env{
							{
								Name:  "PORT",
								Value: "6666",
							},
							{
								Name:  "FOO",
								Value: "bar",
							},
						},
					},
				},
				ketchYamlData: &ketchv1.KetchYamlData{
					Kubernetes: &ketchv1.KetchYamlKubernetesConfig{
						Processes: map[string]ketchv1.KetchYamlProcessConfig{
							"web": ketchv1.KetchYamlProcessConfig{
								Ports: []ketchv1.KetchYamlProcessPortConfig{
									{
										Protocol:   "TCP",
										Port:       8888,
										TargetPort: 6666,
									},
								},
							},
							"worker": ketchv1.KetchYamlProcessConfig{
								Ports: []ketchv1.KetchYamlProcessPortConfig{
									{
										Protocol:   "TCP",
										Port:       8888,
										TargetPort: 6666,
									},
								},
							},
						},
					},
					Hooks: &ketchv1.KetchYamlHooks{
						Restart: ketchv1.KetchYamlRestartHooks{
							Before: []string{"pwd"},
							After:  []string{"echo \"test\""},
						},
					},
				},
				version: strPtr("v1"),
				appType: strPtr("Application"),
			},
		},
		{
			description: "success - defaults",
			yaml: `name: test
framework: myframework
image: gcr.io/kubernetes/sample-app:latest`,
			changeSet: &ChangeSet{
				appName:            "test",
				appUnit:            intPtr(1),
				yamlStrictDecoding: true,
				image:              strPtr("gcr.io/kubernetes/sample-app:latest"),
				framework:          strPtr("myframework"),
				version:            strPtr("v1"),
				appType:            strPtr("Application"),
			},
		},
		{
			description: "validation error - framework",
			yaml: `name: test
image: gcr.io/kubernetes/sample-app:latest`,
			errStr: "missing required field framework",
		},
		{
			description: "validation error - processes without sourcePath",
			yaml: `name: test
framework: myframework
image: gcr.io/kubernetes/sample-app:latest
processes:
  - name: web
    cmd: python app.py`,
			errStr: "running defined processes require a sourcePath",
		},
		{
			description: "success - use appUnits as process.units when units are not specified",
			yaml: `version: v1
type: Application
name: test
image: gcr.io/kubernetes/sample-app:latest
framework: myframework
description: a test
builder: heroku/buildpacks:20
appUnit: 2
processes:
  - name: web
    cmd: python app.py
    units: 1
  - name: worker
    cmd: python app.py`,
			changeSet: &ChangeSet{
				appName:            "test",
				appUnit:            intPtr(2),
				yamlStrictDecoding: true,
				sourcePath:         strPtr("."),
				image:              strPtr("gcr.io/kubernetes/sample-app:latest"),
				description:        strPtr("a test"),
				builder:            strPtr("heroku/buildpacks:20"),
				framework:          strPtr("myframework"),
				processes: &[]ketchv1.ProcessSpec{
					{
						Name:  "web",
						Cmd:   []string{"python", "app.py"},
						Units: intPtr(1),
					},
					{
						Name:  "worker",
						Cmd:   []string{"python", "app.py"},
						Units: intPtr(2),
					},
				},
				version: strPtr("v1"),
				appType: strPtr("Application"),
				ketchYamlData: &ketchv1.KetchYamlData{
					Hooks: &ketchv1.KetchYamlHooks{
						Restart: ketchv1.KetchYamlRestartHooks{},
					},
					Kubernetes: &ketchv1.KetchYamlKubernetesConfig{Processes: map[string]ketchv1.KetchYamlProcessConfig{}},
				},
			},
		},
		{
			description: "success - no cname",
			yaml: `name: test
framework: myframework
image: gcr.io/kubernetes/sample-app:latest
`,
			changeSet: &ChangeSet{
				appName:            "test",
				appUnit:            intPtr(1),
				yamlStrictDecoding: true,
				image:              strPtr("gcr.io/kubernetes/sample-app:latest"),
				framework:          strPtr("myframework"),
				version:            strPtr("v1"),
				appType:            strPtr("Application"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			file, err := os.CreateTemp(t.TempDir(), "*.yaml")
			require.Nil(t, err)
			_, err = file.Write([]byte(tt.yaml))
			require.Nil(t, err)
			defer os.Remove(file.Name())

			cs, err := tt.options.GetChangeSetFromYaml(file.Name())
			if tt.errStr != "" {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), tt.errStr)
			} else {
				require.Nil(t, err)
				require.Equal(t, tt.changeSet, cs)
			}
		})
	}
}
