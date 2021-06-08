package chart

import (
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/utils"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
)

type ApplicationChartV2 struct {
	Templates map[string]interface{}
	Values    map[string]interface{}
	Name      string
}

// NewApplicationChartConfig returns a ChartConfig instance based on the given application.
func NewApplicationChartConfig(application ketchv1.Application) ChartConfig {
	version := fmt.Sprintf("v%v", application.ObjectMeta.Generation)
	chartVersion := fmt.Sprintf("v0.0.%v", application.ObjectMeta.Generation)
	if application.Spec.Version != nil {
		version = *application.Spec.Version
	}
	return ChartConfig{
		Version:     chartVersion,
		Description: application.Spec.Description,
		AppName:     application.Name,
		AppVersion:  version,
	}
}

func NewApplicationChart(application *ketchv1.Application, components map[ketchv1.ComponentType]ketchv1.ComponentSpec) (*ApplicationChartV2, error) {
	templates := make(map[string]interface{})
	componentValues := make(map[string]interface{})
	traitValues := make(map[string]interface{})

	for _, c := range application.Spec.Components {
		component, ok := components[c.Type]
		if !ok {
			return nil, errors.Errorf("component type %s is not defined", c.Type)
		}

		properties := make(map[string]interface{})
		for name, prop := range c.Properties {
			//properties[name], err = yaml.Marshal(prop)
			properties[name] = prop
		}
		componentValues[c.Name] = properties

		componentTemplates, err := RenderComponentTemplates(&component, c.Name)
		if err != nil {
			return nil, err
		}
		for key, value := range componentTemplates {
			templates[key] = value
		}
	}
	return &ApplicationChartV2{
		Values: map[string]interface{}{
			"components": componentValues,
			"traits":     traitValues,
		},
		Templates: templates,
		Name:      application.Name,
	}, nil
}

// RenderComponentTemplates
// for each component.componentSpec.Schematic.Kube.Templates:
// for each FieldPath:
// create nested map; done
// for each template in componentSpec.Schematic.Kube.Templates:
// if value in nested map exists, populate value with {{ directive }}
func RenderComponentTemplates(componentSpec *ketchv1.ComponentSpec, componentName string) (map[string]string, error) {
	templates := make(map[string]string)
	for _, template := range componentSpec.Schematic.Kube.Templates {
		nestedMap := utils.NestedMap{}
		err := yaml.Unmarshal(template.Template.Raw, &nestedMap)
		if err != nil {
			return nil, err
		}

		// TODO this isn't right - figure how to mod helm chart vars:
		//for _, parameter := range template.Parameters {
		//	for _, fieldPath := range parameter.FieldPaths {
		//		key, err := utils.GetNestedMapKeyFromFieldPath(fieldPath)
		//		if err != nil {
		//			return nil, err
		//		}
		//		c, err := nestedMap.Get(key)
		//		if err != nil {
		//			if err == utils.ErrKeyNotFound {
		//				continue
		//			}
		//			return nil, err
		//		}
		//
		//		err = nestedMap.Set(key, fmt.Sprintf(`{{ .Values.%s | default "%v" }}`, parameter.Name, c))
		//		if err != nil {
		//			return nil, err
		//		}
		//	}
		//}
		out, err := yaml.Marshal(nestedMap)
		if err != nil {
			return nil, err
		}
		componentKindIface, err := nestedMap.Get([]interface{}{"kind"})
		if err != nil {
			return nil, err
		}
		componentKind, ok := componentKindIface.(string)
		if !ok {
			return nil, errors.New("component kind is not a string")
		}
		templates[fmt.Sprintf("%s%sComponent.yaml", componentName, componentKind)] = string(out)
	}
	return templates, nil
}

func (c HelmClient) UpdateApplicationChart(appChrt ApplicationChartV2, config ChartConfig, opts ...InstallOption) (*release.Release, error) {
	files, err := appChrt.bufferedFiles(config)
	if err != nil {
		return nil, err
	}
	chrt, err := loader.LoadFiles(files)
	if err != nil {
		return nil, err
	}
	vals, err := appChrt.getValues()
	if err != nil {
		return nil, err
	}
	getValuesClient := action.NewGetValues(c.cfg)
	getValuesClient.AllValues = true
	_, err = getValuesClient.Run(appChrt.Name)
	if err != nil && err.Error() == "release: not found" {
		clientInstall := action.NewInstall(c.cfg)
		clientInstall.ReleaseName = appChrt.Name
		clientInstall.Namespace = c.namespace
		for _, opt := range opts {
			opt(clientInstall)
		}
		return clientInstall.Run(chrt, vals)
	}
	if err != nil {
		return nil, err
	}
	updateClient := action.NewUpgrade(c.cfg)
	updateClient.Namespace = c.namespace
	return updateClient.Run(appChrt.Name, chrt, vals)
}

func (chrt ApplicationChartV2) bufferedFiles(chartConfig ChartConfig) ([]*loader.BufferedFile, error) {
	files := make([]*loader.BufferedFile, 0, len(chrt.Templates)+1)
	for filename, content := range chrt.Templates {
		template, err := yaml.Marshal(content)
		if err != nil {
			return nil, err
		}

		files = append(files, &loader.BufferedFile{
			Name: filepath.Join("templates", filename),
			Data: template,
		})
	}
	valuesBytes, err := yaml.Marshal(chrt.Values)
	if err != nil {
		return nil, err
	}
	files = append(files, &loader.BufferedFile{
		Name: "values.yaml",
		Data: valuesBytes,
	})

	chartYamlContent, err := chartConfig.render()
	if err != nil {
		return nil, err
	}
	files = append(files, &loader.BufferedFile{
		Name: "Chart.yaml",
		Data: chartYamlContent,
	})
	return files, nil
}

func (chrt ApplicationChartV2) getValues() (map[string]interface{}, error) {
	bs, err := yaml.Marshal(chrt.Values)
	if err != nil {
		return nil, err
	}
	vals, err := chartutil.ReadValues(bs)
	if err != nil {
		return nil, err
	}
	return vals, nil
}
