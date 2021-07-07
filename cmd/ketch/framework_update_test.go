package main

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/shipa-corp/ketch/internal/testutils"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/mocks"
)

func Test_frameworkUpdate(t *testing.T) {
	clusterIssuerLe := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cert-manager.io/v1",
			"kind":       "ClusterIssuer",
			"metadata": map[string]interface{}{
				"name": "le-production",
			},
			"spec": map[string]interface{}{
				"acme": "https://acme-v02.api.letsencrypt.org/directory",
			},
		},
	}
	clusterIssuerStaging := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cert-manager.io/v1",
			"kind":       "ClusterIssuer",
			"metadata": map[string]interface{}{
				"name": "le-staging",
			},
			"spec": map[string]interface{}{
				"acme": "https://acme-v02.api.letsencrypt.org/directory",
			},
		},
	}
	frontendFramework := &ketchv1.Framework{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "frontend-framework",
		},
		Spec: ketchv1.FrameworkSpec{
			NamespaceName: "frontend",
			AppQuotaLimit: testutils.IntPtr(30),
			IngressController: ketchv1.IngressControllerSpec{
				ClassName:       "default-classname",
				ServiceEndpoint: "192.168.1.17",
				IngressType:     ketchv1.IstioIngressControllerType,
				ClusterIssuer:   "le-staging",
			},
		},
	}

	tests := []struct {
		name              string
		frameworkName     string
		cfg               config
		options           frameworkUpdateOptions
		yamlData          string
		wantFrameworkSpec ketchv1.FrameworkSpec
		wantOut           string
		wantErr           string
	}{
		{
			name:          "framework from yaml file, ignores flags",
			frameworkName: "frontend-framework",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{frontendFramework},
				DynamicClientObjects: []runtime.Object{clusterIssuerStaging},
			},
			options: frameworkUpdateOptions{
				appQuotaLimit: 10,
			},
			yamlData: `name: frontend-framework
appQuotaLimit: 30
ingressController:
 type: istio
 serviceEndpoint: 192.168.1.18
 clusterIssuer: le-staging
 className: default-classname`,
			wantOut: "Successfully updated!\n",
			wantFrameworkSpec: ketchv1.FrameworkSpec{
				Name:          "frontend-framework",
				Version:       "v1",
				NamespaceName: "ketch-frontend-framework",
				AppQuotaLimit: testutils.IntPtr(30),
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "default-classname",
					ServiceEndpoint: "192.168.1.18",
					IngressType:     ketchv1.IstioIngressControllerType,
					ClusterIssuer:   "le-staging",
				},
			},
		},
		{
			name:          "update service endpoint",
			frameworkName: "frontend-framework",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{frontendFramework},
				DynamicClientObjects: []runtime.Object{clusterIssuerStaging},
			},
			options: frameworkUpdateOptions{
				name:                      "frontend-framework",
				ingressServiceEndpointSet: true,
				ingressServiceEndpoint:    "192.168.1.18",
			},
			wantOut: "Successfully updated!\n",
			wantFrameworkSpec: ketchv1.FrameworkSpec{
				NamespaceName: "frontend",
				AppQuotaLimit: testutils.IntPtr(30),
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "default-classname",
					ServiceEndpoint: "192.168.1.18",
					IngressType:     ketchv1.IstioIngressControllerType,
					ClusterIssuer:   "le-staging",
				},
			},
		},
		{
			name:          "update ingress class name",
			frameworkName: "frontend-framework",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{frontendFramework},
				DynamicClientObjects: []runtime.Object{clusterIssuerStaging},
			},
			options: frameworkUpdateOptions{
				name:                "frontend-framework",
				ingressClassNameSet: true,
				ingressClassName:    "traefik",
			},
			wantOut: "Successfully updated!\n",
			wantFrameworkSpec: ketchv1.FrameworkSpec{
				NamespaceName: "frontend",
				AppQuotaLimit: testutils.IntPtr(30),
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "traefik",
					ServiceEndpoint: "192.168.1.17",
					IngressType:     ketchv1.IstioIngressControllerType,
					ClusterIssuer:   "le-staging",
				},
			},
		},
		{
			name:          "update namespace name",
			frameworkName: "frontend-framework",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{frontendFramework},
				DynamicClientObjects: []runtime.Object{clusterIssuerStaging},
			},
			options: frameworkUpdateOptions{
				name:         "frontend-framework",
				namespaceSet: true,
				namespace:    "new-namespace",
			},
			wantOut: "Successfully updated!\n",
			wantFrameworkSpec: ketchv1.FrameworkSpec{
				NamespaceName: "new-namespace",
				AppQuotaLimit: testutils.IntPtr(30),
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "default-classname",
					ServiceEndpoint: "192.168.1.17",
					IngressType:     ketchv1.IstioIngressControllerType,
					ClusterIssuer:   "le-staging",
				},
			},
		},
		{
			name:          "update app quota",
			frameworkName: "frontend-framework",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{frontendFramework},
				DynamicClientObjects: []runtime.Object{clusterIssuerStaging},
			},
			options: frameworkUpdateOptions{
				name:             "frontend-framework",
				appQuotaLimitSet: true,
				appQuotaLimit:    50,
			},
			wantOut: "Successfully updated!\n",
			wantFrameworkSpec: ketchv1.FrameworkSpec{
				NamespaceName: "frontend",
				AppQuotaLimit: testutils.IntPtr(50),
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "default-classname",
					ServiceEndpoint: "192.168.1.17",
					IngressType:     ketchv1.IstioIngressControllerType,
					ClusterIssuer:   "le-staging",
				},
			},
		},
		{
			name:          "update ingress type",
			frameworkName: "frontend-framework",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{frontendFramework},
				DynamicClientObjects: []runtime.Object{clusterIssuerStaging},
			},
			options: frameworkUpdateOptions{
				name:           "frontend-framework",
				ingressTypeSet: true,
				ingressType:    traefik,
			},
			wantOut: "Successfully updated!\n",
			wantFrameworkSpec: ketchv1.FrameworkSpec{
				NamespaceName: "frontend",
				AppQuotaLimit: testutils.IntPtr(30),
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "default-classname",
					ServiceEndpoint: "192.168.1.17",
					IngressType:     ketchv1.TraefikIngressControllerType,
					ClusterIssuer:   "le-staging",
				},
			},
		},
		{
			name:          "update cluster issuer",
			frameworkName: "frontend-framework",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{frontendFramework},
				DynamicClientObjects: []runtime.Object{clusterIssuerLe},
			},
			options: frameworkUpdateOptions{
				name:                    "frontend-framework",
				ingressClusterIssuerSet: true,
				ingressClusterIssuer:    "le-production",
			},
			wantOut: "Successfully updated!\n",
			wantFrameworkSpec: ketchv1.FrameworkSpec{
				NamespaceName: "frontend",
				AppQuotaLimit: testutils.IntPtr(30),
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "default-classname",
					ServiceEndpoint: "192.168.1.17",
					IngressType:     ketchv1.IstioIngressControllerType,
					ClusterIssuer:   "le-production",
				},
			},
		},
		{
			name:          "err - no framework",
			frameworkName: "frontend-framework",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{},
				DynamicClientObjects: []runtime.Object{clusterIssuerStaging},
			},
			options: frameworkUpdateOptions{
				name:             "frontend-framework",
				appQuotaLimitSet: true,
				appQuotaLimit:    50,
			},
			wantErr: `failed to get the framework: frameworks.theketch.io "frontend-framework" not found`,
		},
		{
			name:          "error - no cluster issuer",
			frameworkName: "frontend-framework",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{frontendFramework},
			},
			options: frameworkUpdateOptions{
				name:                    "frontend-framework",
				ingressClusterIssuerSet: true,
				ingressClusterIssuer:    "le-production",
			},
			wantErr: "cluster issuer not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.yamlData != "" {
				file, err := os.CreateTemp(t.TempDir(), "*.yaml")
				require.Nil(t, err)
				_, err = file.Write([]byte(tt.yamlData))
				require.Nil(t, err)
				defer os.Remove(file.Name())
				tt.options.name = file.Name()
			}
			out := &bytes.Buffer{}
			err := frameworkUpdate(context.Background(), tt.cfg, tt.options, out)
			wantErr := len(tt.wantErr) > 0
			if wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.Equal(t, tt.wantOut, out.String())
			gotFramework := ketchv1.Framework{}
			err = tt.cfg.Client().Get(context.Background(), types.NamespacedName{Name: tt.frameworkName}, &gotFramework)
			require.Nil(t, err)
			require.Equal(t, tt.wantFrameworkSpec, gotFramework.Spec)
		})
	}
}

func TestUpdateFrameworkFromYaml(t *testing.T) {
	frontendFramework := &ketchv1.Framework{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "frontend-framework",
		},
		Spec: ketchv1.FrameworkSpec{
			NamespaceName: "frontend",
			AppQuotaLimit: testutils.IntPtr(30),
			IngressController: ketchv1.IngressControllerSpec{
				ClassName:       "default-classname",
				ServiceEndpoint: "192.168.1.17",
				IngressType:     ketchv1.IstioIngressControllerType,
				ClusterIssuer:   "le-staging",
			},
		},
	}
	tests := []struct {
		name      string
		options   frameworkUpdateOptions
		cfg       config
		yamlData  string
		framework *ketchv1.Framework
	}{
		{
			name:    "success",
			options: frameworkUpdateOptions{},
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{frontendFramework},
			},
			yamlData: `name: frontend-framework
namespace: my-namespace
appQuotaLimit: 5
ingressController:
 type: traefik
 serviceEndpoint: 192.168.1.18
 clusterIssuer: le-production
 className: default-classname`,
			framework: &ketchv1.Framework{
				ObjectMeta: metav1.ObjectMeta{
					Name: "frontend-framework",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Framework",
					APIVersion: "theketch.io/v1beta1",
				},
				Spec: ketchv1.FrameworkSpec{
					Version:       "v1",
					Name:          "frontend-framework",
					NamespaceName: "my-namespace",
					AppQuotaLimit: testutils.IntPtr(5),
					IngressController: ketchv1.IngressControllerSpec{
						IngressType:     "traefik",
						ServiceEndpoint: "192.168.1.18",
						ClusterIssuer:   "le-production",
						ClassName:       "default-classname",
					},
				},
			},
		},
		{
			name:    "success - default version, namespace, appQuotaLimit, and ingress",
			options: frameworkUpdateOptions{},
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{frontendFramework},
			},
			yamlData: `name: frontend-framework`,
			framework: &ketchv1.Framework{
				ObjectMeta: metav1.ObjectMeta{
					Name: "frontend-framework",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Framework",
					APIVersion: "theketch.io/v1beta1",
				},
				Spec: ketchv1.FrameworkSpec{
					Version:       "v1",
					Name:          "frontend-framework",
					NamespaceName: "ketch-frontend-framework",
					AppQuotaLimit: testutils.IntPtr(-1),
					IngressController: ketchv1.IngressControllerSpec{
						IngressType: "traefik",
						ClassName:   "traefik",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.yamlData != "" {
				file, err := os.CreateTemp(t.TempDir(), "*.yaml")
				require.Nil(t, err)
				_, err = file.Write([]byte(tt.yamlData))
				require.Nil(t, err)
				defer os.Remove(file.Name())
				tt.options.name = file.Name()
			}
			res, err := updateFrameworkFromYaml(context.Background(), tt.cfg, tt.options)
			require.Nil(t, err)
			require.Equal(t, tt.framework, res)
		})
	}
}

func TestUpdateFrameworkFromArgs(t *testing.T) {
	frontendFramework := &ketchv1.Framework{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "frontend-framework",
		},
		Spec: ketchv1.FrameworkSpec{
			NamespaceName: "frontend",
			AppQuotaLimit: testutils.IntPtr(30),
			IngressController: ketchv1.IngressControllerSpec{
				ClassName:       "default-classname",
				ServiceEndpoint: "192.168.1.17",
				IngressType:     ketchv1.IstioIngressControllerType,
				ClusterIssuer:   "le-staging",
			},
		},
	}
	tests := []struct {
		name      string
		options   frameworkUpdateOptions
		cfg       config
		framework *ketchv1.Framework
	}{
		{
			name: "success",
			options: frameworkUpdateOptions{
				name:                      "frontend-framework",
				namespace:                 "my-namespace",
				namespaceSet:              true,
				appQuotaLimit:             5,
				appQuotaLimitSet:          true,
				ingressType:               ingressType(1),
				ingressTypeSet:            true,
				ingressServiceEndpoint:    "10.10.20.30",
				ingressServiceEndpointSet: true,
				ingressClassName:          "istio",
				ingressClassNameSet:       true,
				ingressClusterIssuer:      "le-production",
				ingressClusterIssuerSet:   true,
			},
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{frontendFramework},
			},
			framework: &ketchv1.Framework{
				ObjectMeta: metav1.ObjectMeta{
					Name: "frontend-framework",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Framework",
					APIVersion: "theketch.io/v1beta1",
				},
				Spec: ketchv1.FrameworkSpec{
					NamespaceName: "my-namespace",
					AppQuotaLimit: testutils.IntPtr(5),
					IngressController: ketchv1.IngressControllerSpec{
						IngressType:     "istio",
						ServiceEndpoint: "10.10.20.30",
						ClusterIssuer:   "le-production",
						ClassName:       "istio",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := updateFrameworkFromArgs(context.Background(), tt.cfg, tt.options)
			require.Nil(t, err)
			require.Equal(t, tt.framework, res)
		})
	}
}
