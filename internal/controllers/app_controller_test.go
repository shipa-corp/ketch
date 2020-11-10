package controllers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/chart"
	"github.com/shipa-corp/ketch/internal/templates"
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

func (h *helm) UpdateChart(appChrt chart.ApplicationChart, config chart.ChartConfig, opts ...chart.InstallOption) (*release.Release, error) {
	return nil, h.updateChartResults[appChrt.AppName()]
}

func (h *helm) DeleteChart(appName string) error {
	h.deleteChartCalled = append(h.deleteChartCalled, appName)
	return nil
}

func TestAppReconciler_Reconcile(t *testing.T) {

	defaultObjects := []runtime.Object{
		&ketchv1.Pool{
			ObjectMeta: metav1.ObjectMeta{
				Name: "working-pool",
			},
			Spec: ketchv1.PoolSpec{
				NamespaceName: "hello",
				AppQuotaLimit: 100,
				IngressController: ketchv1.IngressControllerSpec{
					IngressType: ketchv1.IstioIngressControllerType,
				},
			},
		},
		&ketchv1.Pool{
			ObjectMeta: metav1.ObjectMeta{
				Name: "second-pool",
			},
			Spec: ketchv1.PoolSpec{
				NamespaceName: "second-namespace",
				AppQuotaLimit: 1,
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
				Pool:        "second-pool",
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
	assert.Nil(t, err)
	defer teardown(ctx)

	tests := []struct {
		name              string
		want              ctrl.Result
		app               ketchv1.App
		wantStatusPhase   ketchv1.AppPhase
		wantStatusMessage string
	}{
		{
			name: "app linked to nonexisting pool",
			app: ketchv1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-1",
				},
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
					Pool:        "non-existing-pool",
				},
			},
			wantStatusPhase:   ketchv1.AppFailed,
			wantStatusMessage: `pool "non-existing-pool" is not found`,
		},
		{
			name: "running application",
			app: ketchv1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-running",
				},
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
					Pool:        "working-pool",
				},
			},
			wantStatusPhase: ketchv1.AppRunning,
		},
		{
			name: "create an app linked to a pool without available slots to run the app",
			app: ketchv1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-3",
				},
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
					Pool:        "second-pool",
				},
			},
			wantStatusPhase:   ketchv1.AppFailed,
			wantStatusMessage: "you have reached the limit of apps",
		},
		{
			name: "app with update-chart-error",
			app: ketchv1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-update-chart-failed",
				},
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
					Pool:        "working-pool",
				},
			},
			wantStatusPhase:   ketchv1.AppPending,
			wantStatusMessage: "failed to update helm chart: render error",
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
				if len(resultApp.Status.Phase) > 0 {
					break
				}
			}
			assert.Equal(t, tt.wantStatusPhase, resultApp.Status.Phase)
			assert.Equal(t, tt.wantStatusMessage, resultApp.Status.Message)

			if resultApp.Status.Phase == ketchv1.AppRunning {
				err = ctx.k8sClient.Delete(context.TODO(), &resultApp)
			}
		})
	}
	assert.Equal(t, []string{"app-running"}, helmMock.deleteChartCalled)
}
