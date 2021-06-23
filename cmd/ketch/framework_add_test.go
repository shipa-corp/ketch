package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/mocks"
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
	file, err := os.CreateTemp("", "*.yaml")
	if err != nil {
		panic(err)
	}
	defer os.Remove(file.Name())
	tests := []struct {
		name          string
		frameworkName string
		cfg           config
		options       frameworkAddOptions
		hasFlags      func() bool

		before            func()
		wantFrameworkSpec ketchv1.FrameworkSpec
		wantOut           string
		wantErr           string
	}{
		{
			name:          "framework from yaml file",
			frameworkName: "hello",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{},
				DynamicClientObjects: []runtime.Object{clusterIssuerLe},
			},
			options: frameworkAddOptions{
				name: file.Name(),
			},
			hasFlags: func() bool {
				return false
			},
			before: func() {
				file.Truncate(0)
				file.Seek(0, 0)
				_, err = io.WriteString(file, `name: hello
appQuotaLimit: 5
ingressController:
 type: istio
 serviceEndpoint: 10.10.20.30
 clusterIssuer: le-production
 className: istio`)
				if err != nil {
					panic(err)
				}
			},
			wantFrameworkSpec: ketchv1.FrameworkSpec{
				NamespaceName: "ketch-hello",
				AppQuotaLimit: 5,
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
			options: frameworkAddOptions{
				name: file.Name(),
			},
			hasFlags: func() bool {
				return false
			},
			before: func() {
				file.Truncate(0)
				file.Seek(0, 0)
				_, err = io.WriteString(file, `appQuotaLimit: 5
ingressController:
  type: istio
  serviceEndpoint: 10.10.20.30
  clusterIssuer: le-production
  className: istio`)
				if err != nil {
					panic(err)
				}
			},
			wantErr: "a framework name is required",
		},
		{
			name:          "framework yaml import errors when flags are specified",
			frameworkName: "",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{},
				DynamicClientObjects: []runtime.Object{clusterIssuerLe},
			},
			options: frameworkAddOptions{
				name: file.Name(),
			},
			hasFlags: func() bool {
				return true
			},
			wantErr: "command line flags are not permitted when passing a framework yaml file",
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
			hasFlags: func() bool {
				return false
			},
			wantFrameworkSpec: ketchv1.FrameworkSpec{
				NamespaceName: "gke",
				AppQuotaLimit: 5,
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
			hasFlags: func() bool {
				return false
			},
			wantFrameworkSpec: ketchv1.FrameworkSpec{
				NamespaceName: "gke",
				AppQuotaLimit: 5,
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
			hasFlags: func() bool {
				return false
			},
			wantFrameworkSpec: ketchv1.FrameworkSpec{
				NamespaceName: "ketch-aws",
				AppQuotaLimit: 5,
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
			hasFlags: func() bool {
				return false
			},
			wantErr: ErrClusterIssuerNotFound.Error(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.before != nil {
				tt.before()
			}
			out := &bytes.Buffer{}
			err := addFramework(context.Background(), tt.cfg, tt.options, out, tt.hasFlags)
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
			addFramework: func(ctx context.Context, cfg config, options frameworkAddOptions, out io.Writer, hasFlags func() bool) error {
				require.False(t, options.ingressClassNameSet)
				require.Equal(t, "gke", options.name)
				return nil
			},
		},
		{
			name: "class name is set",
			args: []string{"ketch", "gke", "--ingress-type", "istio", "--ingress-class-name", "custom-istio"},
			addFramework: func(ctx context.Context, cfg config, options frameworkAddOptions, out io.Writer, hasFlags func() bool) error {
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
	file, err := os.CreateTemp("", "*.yaml")
	if err != nil {
		panic(err)
	}

	tests := []struct {
		name      string
		options   frameworkAddOptions
		before    func()
		framework *ketchv1.Framework
		err       error
	}{
		{
			name: "success",
			options: frameworkAddOptions{
				name: file.Name(),
			},
			before: func() {
				file.Truncate(0)
				file.Seek(0, 0)
				_, err = io.WriteString(file, `name: hello
namespace: my-namespace
appQuotaLimit: 5
ingressController:
 type: istio
 serviceEndpoint: 10.10.20.30
 clusterIssuer: le-production
 className: istio`)
				if err != nil {
					panic(err)
				}
			},
			framework: &ketchv1.Framework{
				ObjectMeta: metav1.ObjectMeta{
					Name: "hello",
				},
				Spec: ketchv1.FrameworkSpec{
					Name:          "hello",
					NamespaceName: "my-namespace",
					AppQuotaLimit: 5,
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
			name: "missing name error",
			options: frameworkAddOptions{
				name: file.Name(),
			},
			before: func() {
				file.Truncate(0)
				file.Seek(0, 0)
				_, err = io.WriteString(file, `appQuotaLimit: 5
ingressController:
 type: istio
 serviceEndpoint: 10.10.20.30
 clusterIssuer: le-production
 className: istio`)
				if err != nil {
					panic(err)
				}
			},
			err: errors.New("a framework name is required"),
		},
		{
			name: "success - default namespace and ingress",
			options: frameworkAddOptions{
				name: file.Name(),
			},
			before: func() {
				file.Truncate(0)
				file.Seek(0, 0)
				_, err = io.WriteString(file, `name: hello
appQuotaLimit: 5`)
				if err != nil {
					panic(err)
				}
			},
			framework: &ketchv1.Framework{
				ObjectMeta: metav1.ObjectMeta{
					Name: "hello",
				},
				Spec: ketchv1.FrameworkSpec{
					Name:          "hello",
					NamespaceName: "ketch-hello",
					AppQuotaLimit: 5,
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
			if tt.before != nil {
				tt.before()
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
					AppQuotaLimit: 5,
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
					AppQuotaLimit: 5,
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
