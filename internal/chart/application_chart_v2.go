package chart

import (
	"fmt"
	"log"
	"path/filepath"
	"strconv"

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/pkg/errors"
	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

// ApplicationChartV2 represents a helm chart's templates and values
type ApplicationChartV2 struct {
	Templates map[string]string
	//Values    map[string]interface{}
	Name string
}

// paramValueSetting represents a value to be injected into a chart at the various fieldpaths
type paramValueSetting struct {
	ValueType  string
	Value      runtime.RawExtension
	FieldPaths []string
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

// NewApplicationChart creates an ApplicationChartV2 from a ketchv1.Application.
// For each ComponentLink specified in the in the Application, it populates a ComponentSpec with the
// properties specified, it renders a template from the ComponentSpec, and it stores the template in a map on the
// ApplicationChart object
func NewApplicationChart(application *ketchv1.Application, components map[ketchv1.ComponentType]ketchv1.ComponentSpec) (*ApplicationChartV2, error) {
	templates := make(map[string]string)
	//componentValues := make(map[string]interface{})
	//traitValues := make(map[string]interface{})

	for _, componentLink := range application.Spec.Components {
		component, ok := components[componentLink.Type]
		if !ok {
			return nil, errors.Errorf("component type %s is not defined", componentLink.Type)
		}

		//properties := make(map[string]interface{})
		//for name, prop := range componentLink.Properties {
		//	properties[name] = prop
		//}
		//componentValues[componentLink.Name] = properties

		componentTemplates, err := RenderComponentTemplates(&component, &componentLink)
		if err != nil {
			return nil, err
		}
		for key, value := range componentTemplates {
			templates[key] = value
		}
	}
	return &ApplicationChartV2{
		//Values: map[string]interface{}{
		//	"components": componentValues,
		//	"traits":     traitValues,
		//},
		Templates: templates,
		Name:      application.Name,
	}, nil
}

// RenderComponentTemplates creates a set of templates, map[componentName] = template.
// It iterates over the componentSpec's templates and assigns parameters from the componentLink (from an Application).
func RenderComponentTemplates(componentSpec *ketchv1.ComponentSpec, componentLink *ketchv1.ComponentLink) (map[string]string, error) {
	templates := make(map[string]string)
	for _, template := range componentSpec.Schematic.Kube.Templates {
		var specMap map[string]interface{}
		err := yaml.Unmarshal(template.Template.Raw, &specMap)
		if err != nil {
			log.Fatal(err)
		}

		raw := unstructured.Unstructured{Object: specMap}
		for _, parameter := range template.Parameters {
			parameterValue, ok := componentLink.Properties[parameter.Name]
			if !ok && parameter.Required {
				return nil, fmt.Errorf("required parameter not found: %s", parameter.Name)
			}
			vals := []paramValueSetting{{
				ValueType:  parameter.Type,
				Value:      parameterValue,
				FieldPaths: parameter.FieldPaths,
			}}

			err = setParameterValuesToKubeObj(&raw, vals)
			if err != nil {
				return nil, err
			}
		}
		templateData, err := yaml.Marshal(raw.Object)
		if err != nil {
			return nil, err
		}
		templates[componentLink.Name] = string(templateData)
	}
	return templates, nil
}

// setParameterValuesToKubeObj assigns []parameterValueSettings to the corresponding fields in an unstructured.Unstructured object
func setParameterValuesToKubeObj(obj *unstructured.Unstructured, values []paramValueSetting) error {
	paved := fieldpath.Pave(obj.Object)
	for _, v := range values {
		for _, f := range v.FieldPaths {
			switch v.ValueType {
			case "string":
				if err := paved.SetString(f, string(v.Value.Raw)); err != nil {
					return err
				}
			case "number":
				fString, err := strconv.ParseFloat(string(v.Value.Raw), 64)
				if err != nil {
					return err
				}
				if err := paved.SetNumber(f, fString); err != nil {
					return err
				}
			case "bool":
				bString, err := strconv.ParseBool(string(v.Value.Raw))
				if err != nil {
					return err
				}
				if err := paved.SetBool(f, bString); err != nil {
					return err
				}
			}
		}
	}
	return nil
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
	//vals, err := appChrt.getValues()
	//if err != nil {
	//	return nil, err
	//}
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
		return clientInstall.Run(chrt, nil)
	}
	if err != nil {
		return nil, err
	}
	updateClient := action.NewUpgrade(c.cfg)
	updateClient.Namespace = c.namespace
	return updateClient.Run(appChrt.Name, chrt, nil)
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
	//valuesBytes, err := yaml.Marshal(chrt.Values)
	//if err != nil {
	//	return nil, err
	//}
	//files = append(files, &loader.BufferedFile{
	//	Name: "values.yaml",
	//	Data: valuesBytes,
	//})

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

//func (chrt ApplicationChartV2) getValues() (map[string]interface{}, error) {
//	bs, err := yaml.Marshal(chrt.Values)
//	if err != nil {
//		return nil, err
//	}
//	vals, err := chartutil.ReadValues(bs)
//	if err != nil {
//		return nil, err
//	}
//	return vals, nil
//}
