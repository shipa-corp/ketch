package chart

import (
	"testing"

	"github.com/stretchr/testify/require"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/utils/conversions"
)

func TestBufferedFiles(t *testing.T) {
	chartConfig := ChartConfig{
		Version:     "v0.0.1",
		Description: "test config",
		AppName:     "test app",
		AppVersion:  "v1",
	}
	templates := map[string]string{
		"test.yaml": "name: {{ App.spec.name }}",
	}
	values := jobValues{
		Job: ketchv1.JobSpec{
			Version:      "v1",
			Type:         "Job",
			Name:         "testjob",
			Framework:    "myframework",
			Description:  "this is a test",
			Parallelism:  2,
			Completions:  2,
			Suspend:      false,
			BackoffLimit: 4,
			Containers: []ketchv1.Container{
				{
					Name:    "test",
					Image:   "ubuntu",
					Command: []string{"pwd"},
				},
			},
			Policy: ketchv1.Policy{
				RestartPolicy: "Never",
			},
		},
	}

	expectedChartYaml := `apiVersion: v2
name: test app
description: test config
type: application
version: v0.0.1
appVersion: v1
`
	expectedTemplates := `name: {{ App.spec.name }}`
	expectedValues := `job:
  backoffLimit: 4
  completions: 2
  containers:
  - command:
    - pwd
    image: ubuntu
    name: test
  description: this is a test
  framework: myframework
  name: testjob
  parallelism: 2
  policy:
    restartPolicy: Never
  type: Job
  version: v1
`

	files, err := bufferedFiles(chartConfig, templates, values)
	require.Nil(t, err)
	require.Equal(t, len(files), 3)
	for _, file := range files {
		switch file.Name {
		case "Chart.yaml":
			require.Equal(t, expectedChartYaml, string(file.Data))
		case "templates/test.yaml":
			require.Equal(t, expectedTemplates, string(file.Data))
		case "values.yaml":
			require.Equal(t, expectedValues, string(file.Data))
		}
	}
}

func TestGetValuesMap(t *testing.T) {
	tests := []struct {
		description string
		i           interface{}
		expected    map[string]interface{}
	}{
		{
			description: "job spec",
			i: ketchv1.JobSpec{
				Version:      "v1",
				Type:         "Job",
				Name:         "testjob",
				Framework:    "myframework",
				Description:  "this is a test",
				Parallelism:  2,
				Completions:  2,
				Suspend:      false,
				BackoffLimit: 4,
				Containers: []ketchv1.Container{
					{
						Name:    "test",
						Image:   "ubuntu",
						Command: []string{"pwd"},
					},
				},
				Policy: ketchv1.Policy{
					RestartPolicy: "Never",
				},
			},
			expected: map[string]interface{}{
				"backoffLimit": 4.,
				"completions":  2.,
				"containers": []interface{}{
					map[string]interface{}{"name": "test", "image": "ubuntu", "command": []interface{}{"pwd"}},
				},
				"description": "this is a test",
				"framework":   "myframework",
				"name":        "testjob",
				"policy":      map[string]interface{}{"restartPolicy": "Never"},
				"type":        "Job",
				"parallelism": 2.,
				"version":     "v1",
			},
		},
		{
			description: "app spec",
			i: ketchv1.AppSpec{
				Version:   conversions.StrPtr("v1"),
				Framework: "myframework",
				Deployments: []ketchv1.AppDeploymentSpec{{
					Image: "test/image",
					Processes: []ketchv1.ProcessSpec{{
						Name: "testprocess",
						Cmd:  []string{"pwd"},
					}},
				}},
			},
			expected: map[string]interface{}{
				"version":   "v1",
				"framework": "myframework",
				"deployments": []interface{}{map[string]interface{}{
					"image": "test/image",
					"processes": []interface{}{map[string]interface{}{
						"name": "testprocess",
						"cmd":  []interface{}{"pwd"},
					}},
					"routingSettings": map[string]interface{}{"weight": 0.},
					"version":         0.,
				}},
				"canary":         map[string]interface{}{},
				"dockerRegistry": map[string]interface{}{},
				"ingress":        map[string]interface{}{"generateDefaultCname": false},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			res, err := getValuesMap(tt.i)
			require.Nil(t, err)
			require.Equal(t, tt.expected, res)
		})
	}
}
