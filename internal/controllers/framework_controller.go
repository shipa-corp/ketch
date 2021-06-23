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

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

// FrameworkReconciler reconciles a Framework object.
type FrameworkReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=theketch.io,resources=frameworks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=theketch.io,resources=frameworks/status,verbs=get;update;patch

func (r *FrameworkReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("framework", req.NamespacedName)

	framework := ketchv1.Framework{}
	if err := r.Get(ctx, req.NamespacedName, &framework); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	status := r.reconcile(ctx, &framework)
	framework.Status = status

	err := r.Status().Update(ctx, &framework)
	return ctrl.Result{}, err
}

func (r *FrameworkReconciler) reconcile(ctx context.Context, framework *ketchv1.Framework) ketchv1.FrameworkStatus {
	frameworks := ketchv1.FrameworkList{}
	err := r.List(ctx, &frameworks)
	if err != nil {
		return ketchv1.FrameworkStatus{
			Phase:     ketchv1.FrameworkFailed,
			Message:   "failed to get a list of frameworks",
			Apps:      framework.Status.Apps,
			Namespace: framework.Status.Namespace,
		}
	}
	namespace := v1.Namespace{}
	failures := 0
	for {
		err := r.Get(ctx, types.NamespacedName{Name: framework.Spec.NamespaceName}, &namespace)
		if err == nil {
			break
		}
		if failures > 10 {
			return ketchv1.FrameworkStatus{
				Phase:     ketchv1.FrameworkFailed,
				Message:   fmt.Sprintf("failed to get %s namespace", framework.Spec.NamespaceName),
				Apps:      framework.Status.Apps,
				Namespace: framework.Status.Namespace,
			}
		}
		if errors.IsNotFound(err) {
			n := &v1.Namespace{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: framework.Spec.NamespaceName,
				},
				Spec:   v1.NamespaceSpec{},
				Status: v1.NamespaceStatus{},
			}
			err = r.Create(ctx, n)
			if err != nil {
				return ketchv1.FrameworkStatus{
					Phase:     ketchv1.FrameworkFailed,
					Message:   fmt.Sprintf("failed to create %s namespace", framework.Spec.NamespaceName),
					Apps:      framework.Status.Apps,
					Namespace: framework.Status.Namespace,
				}
			}
			failures += 1
			time.Sleep(1 * time.Second)
		}
	}
	// we rely on istio automatic sidecar injection
	// https://istio.io/latest/docs/setup/additional-setup/sidecar-injection/#automatic-sidecar-injection
	istioInjectionValue := "disabled"
	if framework.Spec.IngressController.IngressType == ketchv1.IstioIngressControllerType {
		istioInjectionValue = "enabled"
	}
	if namespace.Labels == nil {
		namespace.Labels = map[string]string{}
	}
	namespace.Labels["istio-injection"] = istioInjectionValue

	err = r.Update(ctx, &namespace)
	if err != nil {
		return ketchv1.FrameworkStatus{
			Phase:     ketchv1.FrameworkFailed,
			Message:   fmt.Sprintf("failed to update namespace annotations: %v", err),
			Apps:      framework.Status.Apps,
			Namespace: framework.Status.Namespace,
		}
	}

	ref, err := reference.GetReference(r.Scheme, &namespace)
	if err != nil {
		return ketchv1.FrameworkStatus{
			Phase:     ketchv1.FrameworkFailed,
			Message:   fmt.Sprintf("failed to get a reference to %s namespace", framework.Spec.NamespaceName),
			Apps:      framework.Status.Apps,
			Namespace: framework.Status.Namespace,
		}
	}
	for _, p := range frameworks.Items {
		// It shouldn't happen because we have a web hook that checks this.
		if p.Status.Namespace == nil {
			continue
		}
		if p.Status.Namespace.UID == ref.UID && p.Name != framework.Name {
			return ketchv1.FrameworkStatus{
				Phase:     ketchv1.FrameworkFailed,
				Message:   "Target namespace is already used by another framework",
				Apps:      framework.Status.Apps,
				Namespace: framework.Status.Namespace,
			}
		}
	}
	return ketchv1.FrameworkStatus{
		Namespace: ref,
		Phase:     ketchv1.FrameworkCreated,
		Apps:      framework.Status.Apps,
	}
}

func (r *FrameworkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ketchv1.Framework{}).
		Complete(r)
}
