package chart

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/templates"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNew(t *testing.T) {

	const chartDirectory = "./testdata/charts/"

	frameworkWithClusterIssuer := &ketchv1.Framework{
		ObjectMeta: metav1.ObjectMeta{
			Name: "framework",
		},
		Spec: ketchv1.FrameworkSpec{
			NamespaceName: "ketch-gke",
			IngressController: ketchv1.IngressControllerSpec{
				ClassName:       "ingress-class",
				ServiceEndpoint: "10.10.10.10",
				ClusterIssuer:   "letsencrypt-production",
			},
		},
	}
	frameworkWithoutClusterIssuer := &ketchv1.Framework{
		ObjectMeta: metav1.ObjectMeta{
			Name: "framework",
		},
		Spec: ketchv1.FrameworkSpec{
			NamespaceName: "ketch-gke",
			IngressController: ketchv1.IngressControllerSpec{
				ClassName:       "gke",
				ServiceEndpoint: "20.20.20.20",
			},
		},
	}
	exportedPorts := map[ketchv1.DeploymentVersion][]ketchv1.ExposedPort{
		3: {{Port: 9090, Protocol: "TCP"}},
	}
	dashboard := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dashboard",
		},
		Spec: ketchv1.AppSpec{
			Deployments: []ketchv1.AppDeploymentSpec{
				{
					Image:   "shipasoftware/go-app:v1",
					Version: 3,
					Processes: []ketchv1.ProcessSpec{
						{Name: "web", Units: intRef(3), Cmd: []string{"python"}},
						{Name: "worker", Units: intRef(1), Cmd: []string{"celery"}},
					},
					RoutingSettings: ketchv1.RoutingSettings{
						Weight: 100,
					},
				},
			},
			Env: []ketchv1.Env{
				{Name: "VAR", Value: "VALUE"},
			},
			Framework: "framework",
			Ingress: ketchv1.IngressSpec{
				GenerateDefaultCname: true,
				Cnames:               []string{"theketch.io", "app.theketch.io"},
			},
		},
	}

	tests := []struct {
		name        string
		application *ketchv1.App
		framework   *ketchv1.Framework
		opts        []Option

		wantYamlsFilename string
		wantErr           bool
	}{
		{
			name: "istio templates with cluster issuer",
			opts: []Option{
				WithTemplates(templates.IstioDefaultTemplates),
				WithExposedPorts(exportedPorts),
			},
			application:       dashboard,
			framework:         frameworkWithClusterIssuer,
			wantYamlsFilename: "dashboard-istio-cluster-issuer",
		},
		{
			name: "istio templates without cluster issuer",
			opts: []Option{
				WithTemplates(templates.IstioDefaultTemplates),
				WithExposedPorts(exportedPorts),
			},
			application:       dashboard,
			framework:         frameworkWithoutClusterIssuer,
			wantYamlsFilename: "dashboard-istio",
		},
		{
			name: "traefik templates with cluster issuer",
			opts: []Option{
				WithTemplates(templates.TraefikDefaultTemplates),
				WithExposedPorts(exportedPorts),
			},
			application:       dashboard,
			framework:         frameworkWithClusterIssuer,
			wantYamlsFilename: "dashboard-traefik-cluster-issuer",
		},
		{
			name: "traefik templates without cluster issuer",
			opts: []Option{
				WithTemplates(templates.TraefikDefaultTemplates),
				WithExposedPorts(exportedPorts),
			},
			application:       dashboard,
			framework:         frameworkWithoutClusterIssuer,
			wantYamlsFilename: "dashboard-traefik",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.application, tt.framework, tt.opts...)
			if tt.wantErr {
				require.Nil(t, err, "New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			expectedFilename := filepath.Join(chartDirectory, fmt.Sprintf("%s.yaml", tt.wantYamlsFilename))
			actualFilename := filepath.Join(chartDirectory, fmt.Sprintf("%s-output.yaml", tt.wantYamlsFilename))

			chartConfig := ChartConfig{
				Version: "0.0.1",
				AppName: tt.application.Name,
			}
			client := HelmClient{cfg: &action.Configuration{KubeClient: &fake.PrintingKubeClient{}, Releases: storage.Init(driver.NewMemory())}, namespace: tt.framework.Spec.NamespaceName}

			release, err := client.UpdateChart(*got, chartConfig, func(install *action.Install) {
				install.DryRun = true
				install.ClientOnly = true
			})
			require.Nil(t, err)

			actualManifests := strings.TrimSpace(release.Manifest)
			err = ioutil.WriteFile(actualFilename, []byte(actualManifests), 0755)
			require.Nil(t, err)
			expected, err := ioutil.ReadFile(expectedFilename)
			require.Nil(t, err)
			require.Equal(t, string(expected), actualManifests)
		})
	}
}
