package mocks

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MockClient struct {
	OnList func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error
}

func (m MockClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	panic("implement me")
}

func (m MockClient) List(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
	if m.OnList != nil {
		return m.OnList(ctx, list, opts...)
	}
	return nil
}

func (m MockClient) Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
	panic("implement me")
}

func (m MockClient) Delete(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error {
	panic("implement me")
}

func (m MockClient) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	panic("implement me")
}

func (m MockClient) Patch(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error {
	panic("implement me")
}

func (m MockClient) DeleteAllOf(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error {
	panic("implement me")
}

func (m MockClient) Status() client.StatusWriter {
	panic("implement me")
}

var (
	_ client.Client = &MockClient{}
)
