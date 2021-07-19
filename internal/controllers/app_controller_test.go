package controllers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/release"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/chart"
	"github.com/shipa-corp/ketch/internal/templates"
	"github.com/shipa-corp/ketch/internal/utils/conversions"
)

func stringRef(s string) *string {
	return &s
}

type templateReader struct {
	templatesErrors map[string]error
}

func (t *templateReader) Get(name string) (*templates.Templates, error) {
	err := t.templatesErrors[name]
	if err != nil {
		return nil, err
	}
	return &templates.Templates{}, nil
}

type helm struct {
	updateChartResults map[string]error
	deleteChartCalled  []string
}

func (h *helm) UpdateChart(tv chart.TemplateValuer, config chart.ChartConfig, opts ...chart.InstallOption) (*release.Release, error) {
	return nil, h.updateChartResults[tv.GetName()]
}

func (h *helm) DeleteChart(appName string) error {
	h.deleteChartCalled = append(h.deleteChartCalled, appName)
	return nil
}

func TestAppReconciler_Reconcile(t *testing.T) {

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
		&ketchv1.Framework{
			ObjectMeta: metav1.ObjectMeta{
				Name: "second-framework",
			},
			Spec: ketchv1.FrameworkSpec{
				NamespaceName: "second-namespace",
				AppQuotaLimit: conversions.IntPtr(1),
				IngressController: ketchv1.IngressControllerSpec{
					IngressType: ketchv1.IstioIngressControllerType,
				},
			},
		},
		&ketchv1.App{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default-app",
			},
			Spec: ketchv1.AppSpec{
				Deployments: []ketchv1.AppDeploymentSpec{},
				Framework:   "second-framework",
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
		app                  ketchv1.App
		wantConditionStatus  v1.ConditionStatus
		wantConditionMessage string
	}{
		{
			name: "app linked to nonexisting framework",
			app: ketchv1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-1",
				},
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
					Framework:   "non-existing-framework",
				},
			},
			wantConditionStatus:  v1.ConditionFalse,
			wantConditionMessage: `framework "non-existing-framework" is not found`,
		},
		{
			name: "running application",
			app: ketchv1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-running",
				},
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
					Framework:   "working-framework",
				},
			},
			wantConditionStatus: v1.ConditionTrue,
		},
		{
			name: "create an app linked to a framework without available slots to run the app",
			app: ketchv1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-3",
				},
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
					Framework:   "second-framework",
				},
			},
			wantConditionStatus:  v1.ConditionFalse,
			wantConditionMessage: "you have reached the limit of apps",
		},
		{
			name: "app with update-chart-error",
			app: ketchv1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-update-chart-failed",
				},
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
					Framework:   "working-framework",
				},
			},
			wantConditionStatus:  v1.ConditionFalse,
			wantConditionMessage: "failed to update helm chart: render error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ctx.k8sClient.Create(context.TODO(), &tt.app)
			assert.Nil(t, err)

			resultApp := ketchv1.App{}
			for {
				time.Sleep(250 * time.Millisecond)
				err = ctx.k8sClient.Get(context.TODO(), types.NamespacedName{Name: tt.app.Name}, &resultApp)
				assert.Nil(t, err)
				if len(resultApp.Status.Conditions) > 0 {
					break
				}
			}
			condition := resultApp.Status.Condition(ketchv1.Scheduled)
			require.NotNil(t, condition)
			require.Equal(t, tt.wantConditionStatus, condition.Status)
			assert.Equal(t, tt.wantConditionMessage, condition.Message)

			if condition.Status == v1.ConditionTrue {
				err = ctx.k8sClient.Delete(context.TODO(), &resultApp)
			}
		})
	}
	assert.Equal(t, []string{"app-running"}, helmMock.deleteChartCalled)
}

func TestParseReconcileReason(t *testing.T) {
	tests := []struct {
		msg            string
		expectedErr    string
		expectedString string
	}{
		{
			"app test 1 reconcile",
			"",
			"app test 1 reconcile",
		},
		{
			"app test34s 1 reconcile",
			"",
			"app test34s 1 reconcile",
		},
		{
			"app test-3 1 reconcile",
			"",
			"app test-3 1 reconcile",
		},
		{
			"sdfsdf",
			"unable to parse reconcile reason: input does not match format",
			"",
		},
		{
			"",
			"unable to parse reconcile reason: unexpected EOF",
			"",
		},
		{
			"app test 1 reconcile asdfadfasfasdf 34rt w",
			"",
			"app test 1 reconcile",
		},
		{
			"app zdf dsf reconcile ",
			"unable to parse reconcile reason: expected integer",
			"",
		},
	}

	for _, test := range tests {
		got, err := ParseAppReconcileMessage(test.msg)
		if len(test.expectedErr) > 0 {
			assert.Equal(t, test.expectedErr, err.Error())
		} else {
			assert.Nil(t, err)
			assert.Equal(t, test.expectedString, got.String())
		}
	}
}
