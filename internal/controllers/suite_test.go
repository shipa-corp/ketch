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
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	// +kubebuilder:scaffold:imports

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/templates"
)

type testingContext struct {
	env       *envtest.Environment
	done      chan struct{}
	k8sClient client.Client
}

func setup(reader templates.Reader, helm Helm, objects []runtime.Object) (*testingContext, error) {
	ctx := &testingContext{
		done: make(chan struct{}),
		env: &envtest.Environment{
			CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")},
		},
	}
	cfg, err := ctx.env.Start()
	if err != nil {
		return nil, err
	}
	if err = ketchv1.AddToScheme(scheme.Scheme); err != nil {
		return nil, err
	}
	ctx.k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, err
	}
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, err
	}
	err = (&AppReconciler{
		Client:         k8sManager.GetClient(),
		Log:            ctrl.Log.WithName("controllers").WithName("App"),
		TemplateReader: reader,
		HelmFactoryFn: func(namespace string) (Helm, error) {
			return helm, nil
		},
		Recorder: k8sManager.GetEventRecorderFor("App"),
	}).SetupWithManager(k8sManager)
	if err != nil {
		return nil, err
	}
	err = (&JobReconciler{
		Client:         k8sManager.GetClient(),
		Log:            ctrl.Log.WithName("controllers").WithName("Job"),
		TemplateReader: reader,
		HelmFactoryFn: func(namespace string) (Helm, error) {
			return helm, nil
		},
		Recorder: k8sManager.GetEventRecorderFor("Job"),
	}).SetupWithManager(k8sManager)
	if err != nil {
		return nil, err
	}
	err = (&FrameworkReconciler{
		Client: k8sManager.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Framework"),
	}).SetupWithManager(k8sManager)
	if err != nil {
		return nil, err
	}

	go func() {
		_ = k8sManager.Start(ctx.done)
	}()

	for _, obj := range objects {
		if err = ctx.k8sClient.Create(context.TODO(), obj); err != nil {
			return nil, err
		}
	}

	time.Sleep(5 * time.Second)

	for _, obj := range objects {
		var name string
		switch x := obj.(type) {
		case *ketchv1.Framework:
			name = x.Name
		case *ketchv1.App:
			name = x.Name
		case *ketchv1.Job:
			name = x.Name
		}
		key := types.NamespacedName{Name: name}
		if err = ctx.k8sClient.Get(context.TODO(), key, obj); err != nil {
			return nil, err
		}
		switch x := obj.(type) {
		case *ketchv1.Framework:
			if x.Status.Phase != ketchv1.FrameworkCreated {
				return nil, fmt.Errorf("failed to create %v framework", x.Name)
			}
		case *ketchv1.App:
			if len(x.Status.Conditions) == 0 {
				return nil, fmt.Errorf("failed to run %v app", x.Name)
			}
		case *ketchv1.Job:
			if x.Status.Framework.String() == "" {
				return nil, fmt.Errorf("failed to run %v job", x.Name)
			}
		}
	}
	return ctx, nil
}

func teardown(ctx *testingContext) {
	if ctx == nil {
		return
	}
	ctx.done <- struct{}{}
	err := ctx.env.Stop()
	if err != nil {
		panic(err)
	}
}
