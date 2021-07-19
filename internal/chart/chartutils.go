package chart

import (
	"path/filepath"

	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"sigs.k8s.io/yaml"
)

// bufferedFiles returns a slice of BufferedFile with templates, values, and Chart.yaml
func bufferedFiles(chartConfig ChartConfig, templates map[string]string, values interface{}) ([]*loader.BufferedFile, error) {
	var files []*loader.BufferedFile
	for filename, content := range templates {
		files = append(files, &loader.BufferedFile{
			Name: filepath.Join("templates", filename),
			Data: []byte(content),
		})
	}

	valuesBytes, err := yaml.Marshal(values)
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

// getValuesMap returns a yaml-marshaled map of parameterized object's fields
func getValuesMap(i interface{}) (map[string]interface{}, error) {
	bs, err := yaml.Marshal(i)
	if err != nil {
		return nil, err
	}
	return chartutil.ReadValues(bs)
}
