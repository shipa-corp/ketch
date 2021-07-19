/*


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

// Package controllers contains App and Framework reconcilers to be used with controller-runtime.
package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/release"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/chart"
	"github.com/shipa-corp/ketch/internal/templates"
)

// AppReconciler reconciles a App object.
type AppReconciler struct {
	client.Client
	Log            logr.Logger
	Scheme         *runtime.Scheme
	TemplateReader templates.Reader
	HelmFactoryFn  helmFactoryFn
	Now            timeNowFn
	Recorder       record.EventRecorder
}

// timeNowFn knows how to get the current time.
// Useful for canary deployments using App Reconclier.
type timeNowFn func() time.Time

type helmFactoryFn func(namespace string) (Helm, error)

// Helm has methods to update/delete helm charts.
type Helm interface {
	UpdateChart(tv chart.TemplateValuer, config chart.ChartConfig, opts ...chart.InstallOption) (*release.Release, error)
	DeleteChart(appName string) error
}

// +kubebuilder:rbac:groups=theketch.io,resources=apps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=theketch.io,resources=apps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=theketch.io,resources=frameworks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=theketch.io,resources=frameworks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="networking.k8s.io",resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="networking.istio.io",resources=gateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="networking.istio.io",resources=virtualservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="cert-manager.io",resources=certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="extensions",resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="extensions",resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="traefik.containo.us",resources=ingressroutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="traefik.containo.us",resources=ingressroutes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="traefik.containo.us",resources=traefikservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="traefik.containo.us",resources=traefikservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch;update;delete

func (r *AppReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("app", req.NamespacedName)

	app := ketchv1.App{}
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		if apierrors.IsNotFound(err) {
			err := r.deleteChart(ctx, req.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var (
		err    error
		result ctrl.Result
	)
	scheduleResult := r.reconcile(ctx, &app)
	if scheduleResult.status == v1.ConditionFalse {
		// we have to return an error to run reconcile again.
		err = fmt.Errorf(scheduleResult.message)
		reason := AppReconcileReason{AppName: app.Name, DeploymentCount: app.Spec.DeploymentsCount}
		r.Recorder.Event(&app, v1.EventTypeWarning, reason.String(), err.Error())
	} else {
		app.Status.Framework = scheduleResult.framework
		reason := AppReconcileReason{AppName: app.Name, DeploymentCount: app.Spec.DeploymentsCount}
		r.Recorder.Event(&app, v1.EventTypeNormal, reason.String(), "success")
	}
	app.SetCondition(ketchv1.Scheduled, scheduleResult.status, scheduleResult.message, metav1.NewTime(time.Now()))
	if err := r.Status().Update(context.Background(), &app); err != nil {
		return result, err
	}

	// use canary step interval as the timeout when canary is active
	if app.Spec.Canary.Active {
		result = ctrl.Result{RequeueAfter: app.Spec.Canary.StepTimeInteval}
	}

	if scheduleResult.useTimeout {
		// set default timeout
		result = ctrl.Result{RequeueAfter: reconcileTimeout}
	}

	return result, err
}

type reconcileResult struct {
	status     v1.ConditionStatus
	message    string
	framework  *v1.ObjectReference
	useTimeout bool
}

func (r *AppReconciler) reconcile(ctx context.Context, app *ketchv1.App) reconcileResult {
	framework := ketchv1.Framework{}
	if err := r.Get(ctx, types.NamespacedName{Name: app.Spec.Framework}, &framework); err != nil {
		return reconcileResult{
			status:  v1.ConditionFalse,
			message: fmt.Sprintf(`framework "%s" is not found`, app.Spec.Framework),
		}
	}
	ref, err := reference.GetReference(r.Scheme, &framework)
	if err != nil {
		return reconcileResult{
			status:  v1.ConditionFalse,
			message: err.Error(),
		}
	}
	if framework.Status.Namespace == nil {
		return reconcileResult{
			status:  v1.ConditionFalse,
			message: fmt.Sprintf(`framework "%s" is not linked to a kubernetes namespace`, framework.Name),
		}
	}
	tpls, err := r.TemplateReader.Get(app.TemplatesConfigMapName(framework.Spec.IngressController.IngressType))
	if err != nil {
		return reconcileResult{
			status:  v1.ConditionFalse,
			message: fmt.Sprintf(`failed to read configmap with the app's chart templates: %v`, err),
		}
	}
	if !framework.HasApp(app.Name) && framework.Spec.AppQuotaLimit != nil && len(framework.Status.Apps) >= *framework.Spec.AppQuotaLimit && *framework.Spec.AppQuotaLimit != -1 {
		return reconcileResult{
			status:  v1.ConditionFalse,
			message: fmt.Sprintf(`you have reached the limit of apps`),
		}
	}
	options := []chart.Option{
		chart.WithExposedPorts(app.ExposedPorts()),
		chart.WithTemplates(*tpls),
	}

	appChrt, err := chart.New(app, &framework, options...)
	if err != nil {
		return reconcileResult{
			status:  v1.ConditionFalse,
			message: err.Error(),
		}
	}
	patchedFramework := framework
	if !patchedFramework.HasApp(app.Name) {
		patchedFramework.Status.Apps = append(patchedFramework.Status.Apps, app.Name)
		mergePatch := client.MergeFrom(&framework)
		if err := r.Status().Patch(ctx, &patchedFramework, mergePatch); err != nil {
			return reconcileResult{
				status:  v1.ConditionFalse,
				message: fmt.Sprintf("failed to update framework status: %v", err),
			}
		}
	}
	targetNamespace := framework.Status.Namespace.Name
	helmClient, err := r.HelmFactoryFn(targetNamespace)
	if err != nil {
		return reconcileResult{
			status:  v1.ConditionFalse,
			message: err.Error(),
		}
	}

	// check for canary deployment
	if app.Spec.Canary.Active {
		// ensures that the canary deployment exists
		if len(app.Spec.Deployments) <= 1 {
			// reset canary specs
			app.Spec.Canary = ketchv1.CanarySpec{}

			return reconcileResult{
				status:     v1.ConditionFalse,
				message:    "no canary deployment found",
				useTimeout: true,
			}
		}

		// retry until all pods for canary deployment comes to running state.
		if err := checkPodStatus(r.Client, app.Name, app.Spec.Deployments[1].Version); err != nil {

			if !timeoutExpired(app.Spec.Canary.Started, r.Now()) {
				return reconcileResult{
					status:     v1.ConditionFalse,
					message:    fmt.Sprintf("canary update failed: %v", err),
					useTimeout: true,
				}
			}

			// Do rollback if timeout expired
			app.DoRollback()
			if e := r.Update(ctx, app); err != nil {
				return reconcileResult{
					status:     v1.ConditionFalse,
					message:    fmt.Sprintf("failed to update app crd: %v", e),
					useTimeout: true,
				}
			}
		}

		// Once all pods are running then Perform canary deployment.
		if err = app.DoCanary(metav1.NewTime(r.Now())); err != nil {
			return reconcileResult{
				status:  v1.ConditionFalse,
				message: fmt.Sprintf("canary update failed: %v", err),
			}
		}
		if err := r.Update(ctx, app); err != nil {
			return reconcileResult{
				status:  v1.ConditionFalse,
				message: fmt.Sprintf("canary update failed: %v", err),
			}
		}
	}

	_, err = helmClient.UpdateChart(*appChrt, chart.NewChartConfig(*app))
	if err != nil {
		return reconcileResult{
			status:  v1.ConditionFalse,
			message: fmt.Sprintf("failed to update helm chart: %v", err),
		}
	}
	return reconcileResult{
		framework: ref,
		status:    v1.ConditionTrue,
	}
}

// check if timeout has expired
func timeoutExpired(t *metav1.Time, now time.Time) bool {
	return t.Add(reconcileTimeout).Before(now)
}

// checkPodStatus checks whether all pods for a deployment are running or not.
func checkPodStatus(c client.Client, appName string, depVersion ketchv1.DeploymentVersion) error {
	if c == nil {
		return errors.New("client must be non-nil")
	}

	if len(appName) == 0 || depVersion <= 0 {
		return errors.New("invalid app specifications")
	}

	// podList contains list of Pods matching the specifed labels below
	podList := &v1.PodList{}
	listOpts := []client.ListOption{
		// The specified labels below matches with the required deployment pods of the app.
		client.MatchingLabels(map[string]string{
			"theketch.io/app-name":               appName,
			"theketch.io/app-deployment-version": fmt.Sprintf("%d", depVersion)}),
	}

	if err := c.List(context.Background(), podList, listOpts...); err != nil {
		return err
	}

	// check if all pods are running for the deployment
	for _, pod := range podList.Items {
		// check if pod have voluntarily terminated with a container exit code of 0
		if pod.Status.Phase == v1.PodSucceeded {
			return nil
		}

		if pod.Status.Phase != v1.PodRunning {
			return errors.New("all pods are not running")
		}

		for _, c := range pod.Status.Conditions {
			if c.Status != v1.ConditionTrue {
				return errors.New("all pods are not in healthy state")
			}
		}
	}
	return nil
}

func (r *AppReconciler) deleteChart(ctx context.Context, appName string) error {
	frameworks := ketchv1.FrameworkList{}
	err := r.Client.List(ctx, &frameworks)
	if err != nil {
		return err
	}
	for _, framework := range frameworks.Items {
		if !framework.HasApp(appName) {
			continue
		}

		helmClient, err := r.HelmFactoryFn(framework.Spec.NamespaceName)
		if err != nil {
			return err
		}
		err = helmClient.DeleteChart(appName)
		if err != nil {
			return err
		}
		patchedFramework := framework

		patchedFramework.Status.Apps = make([]string, 0, len(patchedFramework.Status.Apps))
		for _, name := range framework.Status.Apps {
			if name == appName {
				continue
			}
			patchedFramework.Status.Apps = append(patchedFramework.Status.Apps, name)
		}
		mergePatch := client.MergeFrom(&framework)
		return r.Status().Patch(ctx, &patchedFramework, mergePatch)
	}
	return nil

}

func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ketchv1.App{}).
		Complete(r)
}

// AppReconcileReason handle information about app reconcile
type AppReconcileReason struct {
	AppName         string
	DeploymentCount int
}

// String is a Stringer interface implementation
func (r *AppReconcileReason) String() string {
	return fmt.Sprintf(`app %s %d reconcile`, r.AppName, r.DeploymentCount)
}

// ParseAppReconcileMessage makes AppReconcileReason from the incoming event reason string
func ParseAppReconcileMessage(in string) (*AppReconcileReason, error) {
	rm := AppReconcileReason{}
	_, err := fmt.Sscanf(in, `app %s %d reconcile`, &rm.AppName, &rm.DeploymentCount)
	if err != nil {
		return nil, errors.Wrap(err, `unable to parse reconcile reason`)
	}
	return &rm, nil
}
