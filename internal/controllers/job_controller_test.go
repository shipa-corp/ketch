package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/utils/conversions"
)

func TestJobReconciler_Reconcile(t *testing.T) {
	defaultObjects := []runtime.Object{
		&ketchv1.Framework{
			ObjectMeta: metav1.ObjectMeta{
				Name: "working-framework",
			},
			Spec: ketchv1.FrameworkSpec{
				NamespaceName: "hello",
				AppQuotaLimit: conversions.IntPtr(100),
				IngressController: ketchv1.IngressControllerSpec{
					IngressType: ketchv1.IstioIngressControllerType,
				},
			},
		},
	}
	helmMock := &helm{
		updateChartResults: map[string]error{
			"app-update-chart-failed": errors.New("render error"),
		},
	}
	readerMock := &templateReader{
		templatesErrors: map[string]error{
			"templates-failed": errors.New("no templates"),
		},
	}
	ctx, err := setup(readerMock, helmMock, defaultObjects)
	require.Nil(t, err)
	require.NotNil(t, ctx)
	defer teardown(ctx)

	tests := []struct {
		name                 string
		want                 ctrl.Result
		job                  ketchv1.Job
		wantConditionStatus  v1.ConditionStatus
		wantConditionMessage string
	}{
		{
			name: "job linked to nonexisting framework",
			job: ketchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "framework-missing-job",
					Namespace: "default",
				},
				Spec: ketchv1.JobSpec{
					Framework: "non-existent-framework",
				},
			},
			wantConditionStatus:  v1.ConditionFalse,
			wantConditionMessage: `framework "non-existent-framework" is not found`,
		},
		{
			name: "running job",
			job: ketchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
				Spec: ketchv1.JobSpec{
					Framework: "working-framework",
				},
			},
			wantConditionStatus: v1.ConditionTrue,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ctx.k8sClient.Create(context.TODO(), &tt.job)
			assert.Nil(t, err)
			resultJob := ketchv1.Job{}
			for {
				time.Sleep(250 * time.Millisecond)
				err = ctx.k8sClient.Get(context.TODO(), types.NamespacedName{Name: tt.job.Name, Namespace: tt.job.Namespace}, &resultJob)
				assert.Nil(t, err)
				if len(resultJob.Status.Conditions) > 0 {
					break
				}
			}
			condition := resultJob.Status.Condition(ketchv1.Scheduled)
			require.Equal(t, tt.wantConditionStatus, condition.Status)
			require.Equal(t, tt.wantConditionMessage, condition.Message)
		})
	}
}
