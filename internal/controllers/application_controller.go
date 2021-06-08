/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	"helm.sh/helm/v3/pkg/release"

	"k8s.io/apimachinery/pkg/types"

	"github.com/shipa-corp/ketch/internal/chart"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

// ApplicationReconciler reconciles a Application object
type ApplicationReconciler struct {
	client.Client
	Log                logr.Logger
	Scheme             *runtime.Scheme
	HelmChartFactoryFn helmChartFactoryFn
}

type ParamValueSetting struct {
	ketchv1.Parameter
	Value interface{}
}

type helmChartFactoryFn func(namespace string) (Helmer, error)

// Helmer has methods to update/delete helm charts.
type Helmer interface {
	UpdateApplicationChart(appChrt chart.ApplicationChartV2, config chart.ChartConfig, opts ...chart.InstallOption) (*release.Release, error)
	DeleteChart(appName string) error
}

var (
	ErrInvalidParameterType = func(str string) error { return errors.Errorf("invalid parameter type: %s", str) }
)

//+kubebuilder:rbac:groups=resources.my.domain,resources=applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=resources.my.domain,resources=applications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=resources.my.domain,resources=applications/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Application object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.2/pkg/reconcile
func (r *ApplicationReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("application", req.NamespacedName)

	var application ketchv1.Application
	if err := r.Get(ctx, req.NamespacedName, &application); err != nil {
		if apierrors.IsNotFound(err) {
			err = r.deleteChart(ctx, req.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	r.reconcile(ctx, &application)
	return ctrl.Result{}, nil
}

func (r *ApplicationReconciler) reconcile(ctx context.Context, application *ketchv1.Application) ketchv1.ApplicationStatus {
	componentList := ketchv1.ComponentList{}
	if err := r.List(ctx, &componentList); err != nil {
		return ketchv1.ApplicationStatus{
			Status:  v1.ConditionFalse,
			Message: err.Error(),
		}
	}
	components := make(map[ketchv1.ComponentType]ketchv1.ComponentSpec)
	for _, component := range componentList.Items {
		components[ketchv1.ComponentType(component.ObjectMeta.Name)] = component.Spec
	}

	framework := ketchv1.Framework{}
	if err := r.Get(ctx, types.NamespacedName{Name: application.Spec.Framework}, &framework); err != nil {
		return ketchv1.ApplicationStatus{
			Status:  v1.ConditionFalse,
			Message: err.Error(),
		}
	}
	if framework.Status.Namespace == nil {
		return ketchv1.ApplicationStatus{
			Status:  v1.ConditionFalse,
			Message: fmt.Sprintf(`framework "%s" is not linked to a kubernetes namespace`, framework.Name),
		}
	}
	if !framework.HasApp(application.Name) && len(framework.Status.Apps) >= framework.Spec.AppQuotaLimit && framework.Spec.AppQuotaLimit != -1 {
		return ketchv1.ApplicationStatus{
			Status:  v1.ConditionFalse,
			Message: fmt.Sprintf(`you have reached the limit of apps`),
		}
	}
	helmClient, err := r.HelmChartFactoryFn(framework.Status.Namespace.Name)
	if err != nil {
		return ketchv1.ApplicationStatus{
			Status:  v1.ConditionFalse,
			Message: err.Error(),
		}
	}
	appChart, err := chart.NewApplicationChart(application, components)
	if err != nil {
		return ketchv1.ApplicationStatus{
			Status:  v1.ConditionFalse,
			Message: err.Error(),
		}
	}
	_, err = helmClient.UpdateApplicationChart(*appChart, chart.NewApplicationChartConfig(*application))
	if err != nil {
		return ketchv1.ApplicationStatus{
			Status:  v1.ConditionFalse,
			Message: err.Error(),
		}
	}
	return ketchv1.ApplicationStatus{
		Status: v1.ConditionTrue,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ketchv1.Application{}).
		Complete(r)
}

func (r *ApplicationReconciler) deleteChart(ctx context.Context, appName string) error {
	frameworks := ketchv1.FrameworkList{}
	err := r.Client.List(ctx, &frameworks)
	if err != nil {
		return err
	}
	for _, pool := range frameworks.Items {
		if !pool.HasApp(appName) {
			continue
		}

		helmClient, err := r.HelmChartFactoryFn(pool.Spec.NamespaceName)
		if err != nil {
			return err
		}
		err = helmClient.DeleteChart(appName)
		if err != nil {
			return err
		}
		patchedPool := pool

		patchedPool.Status.Apps = make([]string, 0, len(patchedPool.Status.Apps))
		for _, name := range pool.Status.Apps {
			if name == appName {
				continue
			}
			patchedPool.Status.Apps = append(patchedPool.Status.Apps, name)
		}
		mergePatch := client.MergeFrom(&pool)
		return r.Status().Patch(ctx, &patchedPool, mergePatch)
	}
	return nil
}
