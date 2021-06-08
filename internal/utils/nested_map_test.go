package utils

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	tests := []struct {
		n        NestedMap
		keys     []interface{}
		expected interface{}
		err      error
	}{
		{
			n:        NestedMap{"a": map[string]interface{}{"b": "c"}},
			keys:     []interface{}{"a", "b"},
			expected: "c",
		},
		{
			n:        NestedMap{"a": map[string]interface{}{"b": []interface{}{map[string]interface{}{"c": "d"}}}},
			keys:     []interface{}{"a", "b", 0, "c"},
			expected: "d",
		},
		{
			n:        NestedMap{"a": map[string]interface{}{"b": map[string]interface{}{"c": map[string]interface{}{"d": "e", "f": "g"}}}},
			keys:     []interface{}{"a", "b", "c", "f"},
			expected: "g",
		},
		{
			n:        NestedMap{"a": map[string]interface{}{"b": map[string]interface{}{"c": map[string]interface{}{"d": "e", "f": "g"}}}},
			keys:     []interface{}{"a", "b", "c"},
			expected: map[string]interface{}{"d": "e", "f": "g"},
		},
		{
			n:    NestedMap{"a": map[string]interface{}{"b": "c"}},
			keys: []interface{}{"a", "c"},
			err:  ErrKeyNotFound,
		},
		{
			n:    NestedMap{"a": map[string]interface{}{"b": []interface{}{map[string]interface{}{"c": "d"}}}},
			keys: []interface{}{"a", "b", 1, "c"},
			err:  ErrKeyNotFound,
		},
		{
			n:    NestedMap{"a": map[string]interface{}{"b": []interface{}{map[string]interface{}{"c": "d"}}}},
			keys: []interface{}{"a", "b", 1.8, "c"},
			err:  errors.Errorf("invalid key type: %s", "float64"),
		},
	}
	for _, test := range tests {
		value, err := test.n.Get(test.keys)
		if test.err != nil {
			require.EqualError(t, err, test.err.Error())
		} else {
			require.Nil(t, err)
		}
		require.Equal(t, test.expected, value)
	}
}

func TestSet(t *testing.T) {
	tests := []struct {
		n        NestedMap
		keys     []interface{}
		value    map[string]interface{}
		expected NestedMap
		err      error
	}{
		{
			n:        NestedMap{"a": map[string]interface{}{}},
			keys:     []interface{}{"a"},
			value:    map[string]interface{}{"b": "c"},
			expected: NestedMap{"a": map[string]interface{}{"b": "c"}},
		},
		{
			n:        NestedMap{"a": map[string]interface{}{"b": map[string]interface{}{"c": map[string]interface{}{}, "d": map[string]interface{}{}}}},
			keys:     []interface{}{"a", "b", "d"},
			value:    map[string]interface{}{"x": "y"},
			expected: NestedMap{"a": map[string]interface{}{"b": map[string]interface{}{"c": map[string]interface{}{}, "d": map[string]interface{}{"x": "y"}}}},
		},
		{
			n:        NestedMap{"a": map[string]interface{}{"b": map[string]interface{}{"c": []interface{}{map[string]interface{}{"e": map[string]interface{}{}}}, "d": map[string]interface{}{}}}},
			keys:     []interface{}{"a", "b", "c", 0, "e"},
			value:    map[string]interface{}{"x": "y"},
			expected: NestedMap{"a": map[string]interface{}{"b": map[string]interface{}{"c": []interface{}{map[string]interface{}{"e": map[string]interface{}{"x": "y"}}}, "d": map[string]interface{}{}}}},
		},
		{
			n:        NestedMap{"a": map[string]interface{}{"b": map[string]interface{}{"c": map[string]interface{}{}, "d": map[string]interface{}{}}}},
			keys:     []interface{}{"a", "b", "d"},
			value:    map[string]interface{}{"x": "y"},
			expected: NestedMap{"a": map[string]interface{}{"b": map[string]interface{}{"c": map[string]interface{}{}, "d": map[string]interface{}{"x": "y"}}}},
		},
		{
			n:        NestedMap{"a": map[string]interface{}{"b": map[string]interface{}{"c": map[string]interface{}{}, "d": map[string]interface{}{}}}},
			keys:     []interface{}{"a", "c"},
			err:      ErrKeyNotFound,
			expected: NestedMap{"a": map[string]interface{}{"b": map[string]interface{}{"c": map[string]interface{}{}, "d": map[string]interface{}{}}}},
		},
	}
	for _, test := range tests {
		err := test.n.Set(test.keys, test.value)
		if test.err != nil {
			require.EqualError(t, err, test.err.Error())
		} else {
			require.Nil(t, err)
		}
		require.Equal(t, test.expected, test.n)
	}
}

func TestGetNestedMapKeyFromFieldPath(t *testing.T) {
	tests := []struct {
		path     string
		expected []interface{}
	}{
		{
			path:     "spec.template.spec.containers[0].image",
			expected: []interface{}{"spec", "template", "spec", "containers", 0, "image"},
		},
		{
			path:     "spec.template.spec.containers[10].image[2]",
			expected: []interface{}{"spec", "template", "spec", "containers", 10, "image", 2},
		},
		{
			path:     "spec[string].template",
			expected: []interface{}{"spec[string]", "template"},
		},
	}
	for _, test := range tests {
		res, err := GetNestedMapKeyFromFieldPath(test.path)
		require.Nil(t, err)
		require.Equal(t, test.expected, res)
	}
}
