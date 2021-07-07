package main

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	"github.com/shipa-corp/ketch/internal/testutils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/mocks"
)

func Test_frameworkList(t *testing.T) {
	frameworkA := &ketchv1.Framework{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "framework-a",
		},
		Spec: ketchv1.FrameworkSpec{
			NamespaceName: "a",
			AppQuotaLimit: testutils.IntPtr(30),
			IngressController: ketchv1.IngressControllerSpec{
				ClassName:       "istio",
				ServiceEndpoint: "192.168.1.17",
				ClusterIssuer:   "letsencrypt",
				IngressType:     ketchv1.IstioIngressControllerType,
			},
		},
	}
	frameworkB := &ketchv1.Framework{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "framework-b",
		},
		Spec: ketchv1.FrameworkSpec{
			NamespaceName: "b",
			AppQuotaLimit: testutils.IntPtr(30),
			IngressController: ketchv1.IngressControllerSpec{
				ClassName:       "classname-b",
				ServiceEndpoint: "192.168.1.17",
				ClusterIssuer:   "letsencrypt",
				IngressType:     ketchv1.TraefikIngressControllerType,
			},
		},
	}
	tests := []struct {
		name string
		cfg  config

		wantOut string
		wantErr bool
	}{
		{
			name: "update service endpoint",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{frameworkA, frameworkB},
			},
			wantOut: `NAME           STATUS    NAMESPACE    INGRESS TYPE    INGRESS CLASS NAME    CLUSTER ISSUER    APPS
framework-a              a            istio           istio                 letsencrypt       0/30
framework-b              b            traefik         classname-b           letsencrypt       0/30
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			err := frameworkList(context.Background(), tt.cfg, out)
			if (err != nil) != tt.wantErr {
				t.Errorf("frameworkList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotOut := out.String(); gotOut != tt.wantOut {
				t.Errorf("frameworkList() gotOut = \n%v\n, want \n%v\n", gotOut, tt.wantOut)
			}
		})
	}
}

func Test_frameworkListNames(t *testing.T) {
	frameworkA := &ketchv1.Framework{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "framework-a",
		},
	}
	frameworkB := &ketchv1.Framework{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "framework-b",
		},
	}
	tests := []struct {
		name string
		cfg  config

		filter  []string
		want    []string
		wantErr bool
	}{
		{
			name: "no filter show all",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{frameworkA, frameworkB},
			},

			filter:  []string{},
			want:    []string{"framework-a", "framework-b"},
			wantErr: false,
		},
		{
			name: "filtered, show framework-a only",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{frameworkA, frameworkB},
			},

			filter:  []string{"-a"},
			want:    []string{"framework-a"},
			wantErr: false,
		},
		{
			name: "no result, random filter",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{frameworkA, frameworkB},
			},

			filter:  []string{"foo"},
			want:    []string{},
			wantErr: false,
		},
		{
			name: "filtered, random filter and framework-a filter",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{frameworkA, frameworkB},
			},

			filter:  []string{"foo", "-a"},
			want:    []string{"framework-a"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			names, err := frameworkListNames(tt.cfg, tt.filter...)
			if (err != nil) != tt.wantErr {
				t.Errorf("frameworkListNames() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(names, tt.want) {
				t.Errorf("frameworkListNames() got = \n%v\n, want \n%v\n", names, tt.want)
			}
		})
	}
}
