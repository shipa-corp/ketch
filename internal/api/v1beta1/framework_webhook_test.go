package v1beta1

import (
	"context"
	"testing"

	"github.com/shipa-corp/ketch/internal/testutils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/shipa-corp/ketch/internal/api/v1beta1/mocks"
)

func TestFramework_ValidateDelete(t *testing.T) {
	tests := []struct {
		name      string
		framework Framework
		wantErr   error
	}{
		{
			name: "framework with running apps",
			framework: Framework{
				Status: FrameworkStatus{
					Apps: []string{"ketch"},
				},
			},
			wantErr: ErrDeleteFrameworkWithRunningApps,
		},
		{
			name: "framework with no apps",
			framework: Framework{
				Status: FrameworkStatus{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.framework.ValidateDelete(); err != tt.wantErr {
				t.Errorf("ValidateDelete() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

type mockManager struct {
	client *mocks.MockClient
}

func (m *mockManager) GetClient() client.Client {
	return m.client
}

func TestFramework_ValidateCreate(t *testing.T) {

	const listError Error = "error"

	tests := []struct {
		name      string
		framework Framework
		client    *mocks.MockClient
		wantErr   error
	}{
		{
			name: "error getting a list of frameworks",
			client: &mocks.MockClient{
				OnList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					return listError
				},
			},
			wantErr: listError,
		},
		{
			name: "namespace is used",
			client: &mocks.MockClient{
				OnList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					frameworks := list.(*FrameworkList)
					frameworks.Items = []Framework{
						{Spec: FrameworkSpec{NamespaceName: "ketch-namespace"}},
						{Spec: FrameworkSpec{NamespaceName: "theketch-namespace"}},
					}
					return nil
				},
			},
			framework: Framework{
				Spec: FrameworkSpec{
					NamespaceName: "ketch-namespace",
				},
			},
			wantErr: ErrNamespaceIsUsedByAnotherFramework,
		},
		{
			name: "namespace is used",
			client: &mocks.MockClient{
				OnList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					frameworks := list.(*FrameworkList)
					frameworks.Items = []Framework{
						{Spec: FrameworkSpec{NamespaceName: "ketch-namespace"}},
					}
					return nil
				},
			},
			framework: Framework{
				Spec: FrameworkSpec{
					NamespaceName: "theketch-namespace",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frameworkmgr = &mockManager{client: tt.client}
			if err := tt.framework.ValidateCreate(); err != tt.wantErr {
				t.Errorf("ValidateCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFramework_ValidateUpdate(t *testing.T) {

	const listError Error = "error"

	tests := []struct {
		name      string
		framework Framework
		old       runtime.Object
		client    *mocks.MockClient
		wantErr   error
	}{
		{
			name: "error getting a list of frameworks",
			framework: Framework{
				Spec: FrameworkSpec{NamespaceName: "ketch-namespace"},
			},
			client: &mocks.MockClient{
				OnList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					return listError
				},
			},
			old: &Framework{
				Spec: FrameworkSpec{NamespaceName: "theketch-namespace"},
			},
			wantErr: listError,
		},
		{
			name: "namespace is used",
			framework: Framework{
				ObjectMeta: metav1.ObjectMeta{Name: "framework-1"},
				Spec:       FrameworkSpec{NamespaceName: "ketch-namespace"},
				Status:     FrameworkStatus{},
			},
			client: &mocks.MockClient{
				OnList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					frameworks := list.(*FrameworkList)
					frameworks.Items = []Framework{
						{ObjectMeta: metav1.ObjectMeta{Name: "framework-2"}, Spec: FrameworkSpec{NamespaceName: "ketch-namespace"}},
						{ObjectMeta: metav1.ObjectMeta{Name: "framework-1"}, Spec: FrameworkSpec{NamespaceName: "theketch-namespace"}},
					}
					return nil
				},
			},
			old: &Framework{
				Spec: FrameworkSpec{NamespaceName: "theketch-namespace"},
			},
			wantErr: ErrNamespaceIsUsedByAnotherFramework,
		},
		{
			name: "everything is ok",
			framework: Framework{
				ObjectMeta: metav1.ObjectMeta{Name: "framework-1"},
				Spec:       FrameworkSpec{NamespaceName: "ketch-namespace", AppQuotaLimit: testutils.IntPtr(2)},
			},
			client: &mocks.MockClient{
				OnList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					frameworks := list.(*FrameworkList)
					frameworks.Items = []Framework{
						{ObjectMeta: metav1.ObjectMeta{Name: "framework-2"}, Spec: FrameworkSpec{NamespaceName: "namespace"}},
						{ObjectMeta: metav1.ObjectMeta{Name: "framework-1"}, Spec: FrameworkSpec{NamespaceName: "theketch-namespace"}},
					}
					return nil
				},
			},
			old: &Framework{
				Spec: FrameworkSpec{NamespaceName: "theketch-namespace"},
			},
		},
		{
			name: "failed to descrease quota",
			framework: Framework{
				ObjectMeta: metav1.ObjectMeta{Name: "framework-1"},
				Spec:       FrameworkSpec{NamespaceName: "ketch-namespace", AppQuotaLimit: testutils.IntPtr(1)},
				Status: FrameworkStatus{
					Apps: []string{"app-1", "app-2"},
				},
			},
			client: &mocks.MockClient{
				OnList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					frameworks := list.(*FrameworkList)
					frameworks.Items = []Framework{
						{ObjectMeta: metav1.ObjectMeta{Name: "framework-2"}, Spec: FrameworkSpec{NamespaceName: "theketch-namespace"}},
						{ObjectMeta: metav1.ObjectMeta{Name: "framework-1"}, Spec: FrameworkSpec{NamespaceName: "ketch-namespace"}},
					}
					return nil
				},
			},
			old: &Framework{
				Spec: FrameworkSpec{NamespaceName: "ketch-namespace", AppQuotaLimit: testutils.IntPtr(5)},
				Status: FrameworkStatus{
					Apps: []string{"app-1", "app-2"},
				},
			},
			wantErr: ErrDecreaseQuota,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frameworkmgr = &mockManager{client: tt.client}
			if err := tt.framework.ValidateUpdate(tt.old); err != tt.wantErr {
				t.Errorf("ValidateUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
