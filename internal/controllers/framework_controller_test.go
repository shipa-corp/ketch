package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/shipa-corp/ketch/internal/testutils"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

func TestFrameworkReconciler_Reconcile(t *testing.T) {
	defaultObjects := []runtime.Object{
		&ketchv1.Framework{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: "default-framework",
			},
			Spec: ketchv1.FrameworkSpec{
				NamespaceName: "default-namespace",
				AppQuotaLimit: testutils.IntPtr(100),
				IngressController: ketchv1.IngressControllerSpec{
					IngressType: ketchv1.IstioIngressControllerType,
				},
			},
		},
	}
	ctx, err := setup(nil, nil, defaultObjects)
	assert.Nil(t, err)

	defer teardown(ctx)
	tests := []struct {
		name                     string
		framework                ketchv1.Framework
		wantStatusPhase          ketchv1.FrameworkPhase
		wantStatusMessage        string
		wantNamespaceAnnotations map[string]string
	}{
		{
			name: "namespace is used by another framework",
			framework: ketchv1.Framework{
				ObjectMeta: metav1.ObjectMeta{
					Name: "framework-2",
				},
				Spec: ketchv1.FrameworkSpec{
					AppQuotaLimit: testutils.IntPtr(1),
					NamespaceName: "default-namespace",
					IngressController: ketchv1.IngressControllerSpec{
						IngressType: ketchv1.IstioIngressControllerType,
					},
				},
			},
			wantStatusPhase:   ketchv1.FrameworkFailed,
			wantStatusMessage: "Target namespace is already used by another framework",
		},
		{
			name: "istio controller - everything is ok",
			framework: ketchv1.Framework{
				ObjectMeta: metav1.ObjectMeta{
					Name: "framework-3",
				},
				Spec: ketchv1.FrameworkSpec{
					AppQuotaLimit: testutils.IntPtr(1),
					NamespaceName: "another-namespace-3",
					IngressController: ketchv1.IngressControllerSpec{
						IngressType: ketchv1.IstioIngressControllerType,
					},
				},
			},
			wantStatusPhase: ketchv1.FrameworkCreated,
			wantNamespaceAnnotations: map[string]string{
				"istio-injection": "enabled",
			},
		},
		{
			name: "traefik controller - everything is ok",
			framework: ketchv1.Framework{
				ObjectMeta: metav1.ObjectMeta{
					Name: "framework-4",
				},
				Spec: ketchv1.FrameworkSpec{
					AppQuotaLimit: testutils.IntPtr(1),
					NamespaceName: "another-namespace-4",
					IngressController: ketchv1.IngressControllerSpec{
						IngressType: ketchv1.TraefikIngressControllerType,
					},
				},
			},
			wantStatusPhase: ketchv1.FrameworkCreated,
			wantNamespaceAnnotations: map[string]string{
				"istio-injection": "disabled",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := ctx.k8sClient.Create(context.TODO(), &tt.framework)
			assert.Nil(t, err)

			resultFramework := ketchv1.Framework{}
			for {
				time.Sleep(250 * time.Millisecond)
				err = ctx.k8sClient.Get(context.TODO(), types.NamespacedName{Name: tt.framework.Name}, &resultFramework)
				assert.Nil(t, err)
				if len(resultFramework.Status.Phase) > 0 {
					break
				}
			}

			assert.Equal(t, tt.wantStatusPhase, resultFramework.Status.Phase)
			assert.Equal(t, tt.wantStatusMessage, resultFramework.Status.Message)

			if tt.wantStatusPhase == ketchv1.FrameworkCreated {
				assert.NotNil(t, resultFramework.Status.Namespace.Name)
				assert.NotNil(t, resultFramework.Status.Namespace.UID)

				gotNamespace := v1.Namespace{}
				err = ctx.k8sClient.Get(context.TODO(), types.NamespacedName{Name: tt.framework.Spec.NamespaceName}, &gotNamespace)
				assert.Equal(t, tt.wantNamespaceAnnotations, gotNamespace.Labels)
			} else {
				assert.Nil(t, resultFramework.Status.Namespace)
			}
		})
	}
}
