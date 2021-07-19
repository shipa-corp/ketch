package chart

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/templates"
)

var (
	testJob = &ketchv1.Job{
		ObjectMeta: v1.ObjectMeta{
			Generation: 1,
		},
		Spec: ketchv1.JobSpec{
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
)

func TestNewJobChart(t *testing.T) {
	options := []Option{
		func(opts *Options) {
			opts.Templates = templates.Templates{Yamls: map[string]string{"test.yaml": "Lots of values"}}
		},
	}
	expected := &JobChart{
		values: jobValues{
			Job: testJob.Spec,
		},
		templates: map[string]string{"test.yaml": "Lots of values"},
	}
	jobChart := NewJobChart(testJob, options...)
	require.Equal(t, expected, jobChart)
}

func TestNewJobChartConfig(t *testing.T) {
	expected := ChartConfig{
		Version:     fmt.Sprintf("v0.0.%v", testJob.ObjectMeta.Generation),
		Description: testJob.Spec.Description,
		AppName:     testJob.Spec.Name,
		AppVersion:  testJob.Spec.Version,
	}
	chartConfig := NewJobChartConfig(*testJob)
	require.Equal(t, expected, chartConfig)
}
