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
	temp, err := os.CreateTemp("", "*.yaml")
	require.Nil(t, err)
	defer os.Remove(temp.Name())

	tests := []struct {
		description string
		yaml        string
		options     *Options
		changeSet   *ChangeSet
	}{
		{
			description: "success",
			yaml: `version: v1
type: Application
name: test
image: gcr.io/kubernetes-312803/sample-go-app:latest
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
        protocol: TCP`,
			options: &Options{},
			changeSet: &ChangeSet{
				appName:              "test",
				appUnit:              intPtr(1),
				yamlStrictDecoding:   true,
				sourcePath:           strPtr("."),
				image:                strPtr("gcr.io/kubernetes-312803/sample-go-app:latest"),
				description:          strPtr("a test"),
				envs:                 &[]string{"PORT=6666", "FOO=bar"},
				framework:            strPtr("myframework"),
				dockerRegistrySecret: strPtr(""),
				builder:              strPtr("heroku/buildpacks:20"),
				buildPacks:           &[]string{"test-buildpack"},
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
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			temp.Truncate(0)
			temp.Seek(0, 0)
			temp.Write([]byte(tt.yaml))
			cs, err := tt.options.GetChangeSetFromYaml(temp.Name())
			require.Nil(t, err)
			require.Equal(t, tt.changeSet, cs)
		})
	}
}
