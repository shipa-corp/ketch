// Package templates provides a way to load and store an application's helm chart templates.
//
// Currently, an app's templates are stored as a ConfigMap including templates provided by ketch by default.
//
// apiVersion: v1
// kind: ConfigMap
// metadata:
//   name: <name>
//   namespace: ketch-system
// data:
//   services.yaml: |-
//     ..
//   deployments.yaml: |-
//     ..
package templates

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Templates represents a helm chart's "templates/" folder.
type Templates struct {

	// Yamls contains a content for each yaml file in a helm chart's "templates/" folder.
	Yamls map[string]string
}

// Storage is an implementation of Client interface.
type Storage struct {
	client    client.Client
	namespace string
}

// Updater knows how to update and delete templates.
type Updater interface {
	Update(name string, templates Templates) error
}

// Reader knows how to get templates.
type Reader interface {
	Get(name string) (*Templates, error)
}

// Client knows how to read and update templates stored in a kubernetes cluster.
type Client interface {
	Reader
	Updater
}

// NewStorage returns a Storage instance.
func NewStorage(client client.Client, namespace string) *Storage {
	return &Storage{
		client:    client,
		namespace: namespace,
	}
}

var (
	IstioDefaultTemplates = Templates{
		Yamls: GeneratedYamls.IstioYamls,
	}
	TraefikDefaultTemplates = Templates{
		Yamls: GeneratedYamls.TraefikYamls,
	}
	NoIngressTemplates = Templates{
		Yamls: GeneratedYamls.NoIngressYamls,
	}
)

// IngressConfigMapName returns a name of a configmap to store the ingress' templates to render helm chart.
func IngressConfigMapName(ingress string) string {
	return fmt.Sprintf("ingress-%s-templates", ingress)
}

// Get returns templates stored in a configmap with the provided name.
func (s *Storage) Get(name string) (*Templates, error) {
	ctx := context.TODO()
	cm := v1.ConfigMap{}
	namespacedName := types.NamespacedName{Name: name, Namespace: s.namespace}
	if err := s.client.Get(ctx, namespacedName, &cm); err != nil {
		return nil, err
	}
	return &Templates{Yamls: cm.Data}, nil
}

// Update creates or updates a configmap with the new templates.
func (s *Storage) Update(name string, templates Templates) error {
	namespacedName := types.NamespacedName{Name: name, Namespace: s.namespace}
	ctx := context.TODO()
	cm := v1.ConfigMap{}
	if err := s.client.Get(ctx, namespacedName, &cm); err != nil {
		if apierrors.IsNotFound(err) {
			return s.client.Create(ctx, templates.toConfigMap(name, s.namespace))
		}
	}
	return s.client.Update(ctx, templates.toConfigMap(name, s.namespace))
}

func (tpl Templates) toConfigMap(name string, namespace string) *v1.ConfigMap {
	cm := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: make(map[string]string, len(tpl.Yamls)),
	}
	for name, value := range tpl.Yamls {
		cm.Data[name] = value
	}
	return &cm
}

// ReadDirectory reads files in the directory and returns a Templates instance populated with the files.
func ReadDirectory(directory string) (*Templates, error) {
	infos, err := ioutil.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	templates := Templates{
		Yamls: make(map[string]string, len(infos)),
	}
	for _, info := range infos {
		if !info.IsDir() {
			fileName := filepath.Join(directory, info.Name())
			content, err := ioutil.ReadFile(fileName)
			if err != nil {
				return nil, err
			}
			templates.Yamls[info.Name()] = string(content)
		}
	}
	return &templates, nil
}
