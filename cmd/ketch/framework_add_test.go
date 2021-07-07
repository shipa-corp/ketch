package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/mocks"
	"github.com/shipa-corp/ketch/internal/testutils"
)

func Test_addFramework(t *testing.T) {
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

	tests := []struct {
		name              string
		frameworkName     string
		cfg               config
		options           frameworkAddOptions
		yamlData          string
		wantFrameworkSpec ketchv1.FrameworkSpec
		wantOut           string
		wantErr           string
	}{
		{
			name:          "framework from yaml file, ignores flags",
			frameworkName: "hello",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{},
				DynamicClientObjects: []runtime.Object{clusterIssuerLe},
			},
			options: frameworkAddOptions{
				appQuotaLimit: 10,
			},
			yamlData: `name: hello
ingressController:
  type: istio
  serviceEndpoint: 10.10.20.30
  clusterIssuer: le-production
  className: istio`,
			wantFrameworkSpec: ketchv1.FrameworkSpec{
				Name:          "hello",
				Version:       "v1",
				NamespaceName: "ketch-hello",
				AppQuotaLimit: testutils.IntPtr(-1),
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "istio",
					ServiceEndpoint: "10.10.20.30",
					IngressType:     ketchv1.IstioIngressControllerType,
					ClusterIssuer:   "le-production",
				},
			},
			wantOut: "Successfully added!\n",
		},
		{
			name:          "framework yaml missing name",
			frameworkName: "",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{},
				DynamicClientObjects: []runtime.Object{clusterIssuerLe},
			},
			options: frameworkAddOptions{},
			yamlData: `appQuotaLimit: 5
ingressController:
  type: istio  
  serviceEndpoint: 10.10.20.30
  clusterIssuer: le-production
  className: istio`,
			wantErr: "a framework name is required",
		},
		{
			name:          "default class name for istio is istio",
			frameworkName: "hello",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{},
				DynamicClientObjects: []runtime.Object{clusterIssuerLe},
			},
			options: frameworkAddOptions{
				name:                   "hello",
				appQuotaLimit:          5,
				namespace:              "gke",
				ingressServiceEndpoint: "10.10.20.30",
				ingressType:            istio,
				ingressClusterIssuer:   "le-production",
			},
			wantFrameworkSpec: ketchv1.FrameworkSpec{
				NamespaceName: "gke",
				AppQuotaLimit: testutils.IntPtr(5),
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "istio",
					ServiceEndpoint: "10.10.20.30",
					IngressType:     ketchv1.IstioIngressControllerType,
					ClusterIssuer:   "le-production",
				},
			},
			wantOut: "Successfully added!\n",
		},
		{
			name:          "successfully added with istio",
			frameworkName: "hello",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{},
				DynamicClientObjects: []runtime.Object{clusterIssuerLe},
			},
			options: frameworkAddOptions{
				name:                   "hello",
				appQuotaLimit:          5,
				namespace:              "gke",
				ingressClassNameSet:    true,
				ingressClassName:       "custom-class-name",
				ingressServiceEndpoint: "10.10.20.30",
				ingressType:            istio,
				ingressClusterIssuer:   "le-production",
			},
			wantFrameworkSpec: ketchv1.FrameworkSpec{
				NamespaceName: "gke",
				AppQuotaLimit: testutils.IntPtr(5),
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "custom-class-name",
					ServiceEndpoint: "10.10.20.30",
					IngressType:     ketchv1.IstioIngressControllerType,
					ClusterIssuer:   "le-production",
				},
			},
			wantOut: "Successfully added!\n",
		},
		{
			name:          "traefik + default namespace with ketch- prefix",
			frameworkName: "aws",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{},
			},
			options: frameworkAddOptions{
				name:                   "aws",
				appQuotaLimit:          5,
				ingressClassName:       "traefik",
				ingressServiceEndpoint: "10.10.10.10",
				ingressType:            traefik,
			},
			wantFrameworkSpec: ketchv1.FrameworkSpec{
				NamespaceName: "ketch-aws",
				AppQuotaLimit: testutils.IntPtr(5),
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "traefik",
					ServiceEndpoint: "10.10.10.10",
					IngressType:     ketchv1.TraefikIngressControllerType,
				},
			},
			wantOut: "Successfully added!\n",
		},
		{
			name: "error - no cluster issuer",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{},
				DynamicClientObjects: []runtime.Object{},
			},
			options: frameworkAddOptions{
				name:                 "hello",
				ingressClusterIssuer: "le-production",
			},
			wantErr: ErrClusterIssuerNotFound.Error(),
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
			err := addFramework(context.Background(), tt.cfg, tt.options, out)
			if len(tt.wantErr) > 0 {
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

func Test_newFrameworkAddCmd(t *testing.T) {
	pflag.CommandLine = pflag.NewFlagSet("ketch", pflag.ExitOnError)

	tests := []struct {
		name         string
		args         []string
		addFramework addFrameworkFn
		wantErr      bool
	}{
		{
			name: "class name is not set",
			args: []string{"ketch", "gke", "--ingress-type", "istio"},
			addFramework: func(ctx context.Context, cfg config, options frameworkAddOptions, out io.Writer) error {
				require.False(t, options.ingressClassNameSet)
				require.Equal(t, "gke", options.name)
				return nil
			},
		},
		{
			name: "class name is set",
			args: []string{"ketch", "gke", "--ingress-type", "istio", "--ingress-class-name", "custom-istio"},
			addFramework: func(ctx context.Context, cfg config, options frameworkAddOptions, out io.Writer) error {
				require.True(t, options.ingressClassNameSet)
				require.Equal(t, "gke", options.name)
				require.Equal(t, "custom-istio", options.ingressClassName)
				return nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Args = tt.args
			cmd := newFrameworkAddCmd(nil, nil, tt.addFramework)
			err := cmd.Execute()
			if tt.wantErr {
				require.NotNil(t, err)
				return
			}
			require.Nil(t, err)
		})
	}
}

func TestNewFrameworkFromYaml(t *testing.T) {
	tests := []struct {
		name      string
		options   frameworkAddOptions
		yamlData  string
		framework *ketchv1.Framework
		err       error
	}{
		{
			name:    "success",
			options: frameworkAddOptions{},
			yamlData: `name: hello
namespace: my-namespace
appQuotaLimit: 5
ingressController:
 type: istio
 serviceEndpoint: 10.10.20.30
 clusterIssuer: le-production
 className: istio`,
			framework: &ketchv1.Framework{
				ObjectMeta: metav1.ObjectMeta{
					Name: "hello",
				},
				Spec: ketchv1.FrameworkSpec{
					Version:       "v1",
					Name:          "hello",
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
		{
			name:    "missing name error",
			options: frameworkAddOptions{},
			yamlData: `appQuotaLimit: 5
ingressController:
 type: istio
 serviceEndpoint: 10.10.20.30
 clusterIssuer: le-production
 className: istio`,
			err: errors.New("a framework name is required"),
		},
		{
			name:     "success - default version, namespace, appQuotaLimit, and ingress",
			options:  frameworkAddOptions{},
			yamlData: `name: hello`,
			framework: &ketchv1.Framework{
				ObjectMeta: metav1.ObjectMeta{
					Name: "hello",
				},
				Spec: ketchv1.FrameworkSpec{
					Version:       "v1",
					Name:          "hello",
					NamespaceName: "ketch-hello",
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
			res, err := newFrameworkFromYaml(tt.options)
			if tt.err != nil {
				require.Equal(t, tt.err, err)
			} else {
				require.Nil(t, err)
			}
			require.Equal(t, tt.framework, res)
		})
	}
}

func TestNewFrameworkFromArgs(t *testing.T) {
	tests := []struct {
		name      string
		options   frameworkAddOptions
		framework *ketchv1.Framework
	}{
		{
			name: "success",
			options: frameworkAddOptions{
				name:                   "hello",
				namespace:              "my-namespace",
				appQuotaLimit:          5,
				ingressType:            ingressType(1),
				ingressServiceEndpoint: "10.10.20.30",
				ingressClassName:       "istio",
				ingressClusterIssuer:   "le-production",
				ingressClassNameSet:    true,
			},
			framework: &ketchv1.Framework{
				ObjectMeta: metav1.ObjectMeta{
					Name: "hello",
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
		{
			name: "success - default namespace and ingress",
			options: frameworkAddOptions{
				name:          "hello",
				appQuotaLimit: 5,
			},
			framework: &ketchv1.Framework{
				ObjectMeta: metav1.ObjectMeta{
					Name: "hello",
				},
				Spec: ketchv1.FrameworkSpec{
					NamespaceName: "ketch-hello",
					AppQuotaLimit: testutils.IntPtr(5),
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
			res := newFrameworkFromArgs(tt.options)
			require.Equal(t, tt.framework, res)
		})
	}
}
