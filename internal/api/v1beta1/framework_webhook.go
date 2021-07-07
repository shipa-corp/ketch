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

package v1beta1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var frameworklog = logf.Log.WithName("framework-resource")

type manager interface {
	GetClient() client.Client
}

var frameworkmgr manager = nil

func (r *Framework) SetupWebhookWithManager(mgr ctrl.Manager) error {
	frameworkmgr = mgr
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-theketch-io-v1beta1-framework,mutating=true,failurePolicy=fail,groups=theketch.io,resources=frameworks,verbs=create;update,versions=v1beta1,name=mframework.kb.io

var _ webhook.Defaulter = &Framework{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Framework) Default() {
	frameworklog.Info("default", "name", r.Name)
}

// +kubebuilder:webhook:verbs=create;update;delete,path=/validate-theketch-io-v1beta1-framework,mutating=false,failurePolicy=fail,groups=theketch.io,resources=frameworks,versions=v1beta1,name=vframework.kb.io

var _ webhook.Validator = &Framework{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Framework) ValidateCreate() error {
	frameworklog.Info("validate create", "name", r.Name)
	client := frameworkmgr.GetClient()
	ctx := context.TODO()
	frameworks := FrameworkList{}
	if err := client.List(ctx, &frameworks); err != nil {
		return err
	}
	for _, framework := range frameworks.Items {
		if framework.Spec.NamespaceName == r.Spec.NamespaceName {
			return ErrNamespaceIsUsedByAnotherFramework
		}
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Framework) ValidateUpdate(old runtime.Object) error {
	frameworklog.Info("validate update", "name", r.Name)

	oldFramework, ok := old.(*Framework)
	if !ok {
		return fmt.Errorf("can't validate framework update")
	}

	c := frameworkmgr.GetClient()
	if oldFramework.Spec.NamespaceName != r.Spec.NamespaceName {
		if len(r.Status.Apps) > 0 {
			return ErrChangeNamespaceWhenAppsRunning
		}
		frameworks := FrameworkList{}
		if err := c.List(context.Background(), &frameworks); err != nil {
			return err
		}
		for _, framework := range frameworks.Items {
			if framework.Spec.NamespaceName == r.Spec.NamespaceName && r.Name != framework.Name {
				return ErrNamespaceIsUsedByAnotherFramework
			}
		}
	}

	if oldFramework.Spec.AppQuotaLimit != nil && r.Spec.AppQuotaLimit != nil && *oldFramework.Spec.AppQuotaLimit != *r.Spec.AppQuotaLimit {
		if r.Spec.AppQuotaLimit != nil && *r.Spec.AppQuotaLimit < len(r.Status.Apps) && *r.Spec.AppQuotaLimit != -1 {
			return ErrDecreaseQuota
		}
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Framework) ValidateDelete() error {
	frameworklog.Info("validate delete", "name", r.Name)
	if len(r.Status.Apps) > 0 {
		return ErrDeleteFrameworkWithRunningApps
	}
	return nil
}
