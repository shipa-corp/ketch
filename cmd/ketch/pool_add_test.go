package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/mocks"
)

func Test_addPool(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config
		options poolAddOptions

		wantPoolSpec ketchv1.PoolSpec
		wantOut      string
		wantErr      bool
	}{
		{
			name: "successfully added with istio",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{},
			},
			options: poolAddOptions{
				name:                   "hello",
				appQuotaLimit:          5,
				kubeNamespace:          "gke",
				ingressClassName:       "istio",
				ingressDomainName:      "theketch.cloud",
				ingressServiceEndpoint: "10.10.20.30",
				ingressType:            istio,
			},

			wantPoolSpec: ketchv1.PoolSpec{
				NamespaceName: "gke",
				AppQuotaLimit: 5,
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "istio",
					Domain:          "theketch.cloud",
					ServiceEndpoint: "10.10.20.30",
					IngressType:     ketchv1.IstioIngressControllerType,
				},
			},
			wantOut: "Successfully added!\n",
		},
		{
			name: "successfully added with traefik17",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{},
			},
			options: poolAddOptions{
				name:                   "hello",
				appQuotaLimit:          5,
				kubeNamespace:          "gke",
				ingressClassName:       "traefik",
				ingressDomainName:      "theketch.io",
				ingressServiceEndpoint: "10.10.10.10",
				ingressType:            traefik17,
			},

			wantPoolSpec: ketchv1.PoolSpec{
				NamespaceName: "gke",
				AppQuotaLimit: 5,
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "traefik",
					Domain:          "theketch.io",
					ServiceEndpoint: "10.10.10.10",
					IngressType:     ketchv1.Traefik17IngressControllerType,
				},
			},
			wantOut: "Successfully added!\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			err := addPool(context.Background(), tt.cfg, tt.options, out)
			if (err != nil) != tt.wantErr {
				t.Errorf("addPool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotOut := out.String(); gotOut != tt.wantOut {
				t.Errorf("addPool() gotOut = %v, want %v", gotOut, tt.wantOut)
			}
			gotPool := ketchv1.Pool{}
			err = tt.cfg.Client().Get(context.Background(), types.NamespacedName{Name: tt.options.name}, &gotPool)
			assert.Nil(t, err)
			if diff := cmp.Diff(gotPool.Spec, tt.wantPoolSpec); diff != "" {
				t.Errorf("PoolSpec mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
