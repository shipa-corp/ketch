package utils

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// NestedMap is a struct that allows Inserting and Getting objects from an undefined structure.
// The key must be a string. Subsequent nested interfaces may be type:
// map[string]interface{}
// []interface{} containing map[string]interface{}
// Useful for digging into nested, unstructured types from yaml parsing.
type NestedMap map[string]interface{}

type NestedMapSlice []interface{}

var (
	ErrKeyNotFound = errors.New("key not found")
)

// Get retrieves a nested value defined by the keys slice
func (n NestedMap) Get(keys []interface{}) (interface{}, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	firstKey, ok := keys[0].(string)
	if !ok {
		return nil, errors.New("first key must be a string")
	}
	value := n[firstKey]
	if len(keys) == 1 {
		return value, nil
	}
	for _, key := range keys[1:] {
		switch key.(type) {
		case string:
			fmt.Println("V", value, reflect.TypeOf(value))
			value, ok = value.(map[string]interface{})[key.(string)]
			if !ok {
				return nil, ErrKeyNotFound
			}
		case int:
			if len(value.([]interface{}))-1 < key.(int) {
				return nil, ErrKeyNotFound
			}
			value = value.([]interface{})[key.(int)].(map[string]interface{})

		default:
			return nil, errors.Errorf("invalid key type: %s", reflect.TypeOf(key))
		}
	}
	return value, nil
}

// Set sets a value defined by the keys slice
func (n NestedMap) Set(keys []interface{}, value interface{}) error {
	if next, ok := n[keys[0].(string)]; !ok {
		return ErrKeyNotFound
	} else {
		if len(keys) == 1 {
			n[keys[0].(string)] = value
			return nil
		}
		switch next.(type) {
		case map[string]interface{}:
			return NestedMap(next.(map[string]interface{})).Set(keys[1:], value)
		case []interface{}:
			return NestedMapSlice(next.([]interface{})).Set(keys[1:], value)
		default:
			return errors.Errorf("invalid value type: %s", reflect.TypeOf(next))
		}
	}
	return nil
}

func (n NestedMapSlice) Set(keys []interface{}, value interface{}) error {
	if len(n)-1 < keys[0].(int) {
		return ErrKeyNotFound
	}
	next := n[keys[0].(int)]
	if len(keys) == 1 {
		newValue, ok := value.(NestedMap)
		if !ok {
			return errors.New("value must be map[string]interface{]")
		}
		n[keys[0].(int)] = newValue
		return nil
	}
	return NestedMap(next.(map[string]interface{})).Set(keys[1:], value)
}

// GetNestedMapKeyFromFieldPath parses a path, e.g. ".spec.template[0].values",
// returning a key slice, e.g. ["spec", "template", 0, "values"].
func GetNestedMapKeyFromFieldPath(path string) ([]interface{}, error) {
	var key []interface{}
	arr := strings.Split(path, ".")
	indexRegex := regexp.MustCompile(`\[[0-9]*\]`)
	for _, str := range arr {
		indexString := indexRegex.FindString(str)
		if indexString == "" {
			key = append(key, str)
			continue
		}

		key = append(key, strings.Replace(str, indexString, "", 1))
		index, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSuffix(indexString, "]"), "["))
		if err != nil {
			return nil, err
		}
		key = append(key, index)
	}
	return key, nil
}
