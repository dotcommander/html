package render

import (
	"fmt"
	"reflect"
)

// strictTemplateIndex preserves template indexing but rejects absent map keys.
// The standard index function ignores Option("missingkey=error"), so otherwise
// a key's spelling (dot access vs index) would silently change the contract.
func strictTemplateIndex(item any, indexes ...any) (any, error) {
	value := reflect.ValueOf(item)
	for _, index := range indexes {
		for value.IsValid() && (value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer) {
			value = value.Elem()
		}
		if !value.IsValid() {
			return nil, fmt.Errorf("cannot index nil")
		}
		key := reflect.ValueOf(index)
		switch value.Kind() {
		case reflect.Map:
			if !key.IsValid() || !key.Type().AssignableTo(value.Type().Key()) {
				return nil, fmt.Errorf("invalid map index type %T", index)
			}
			value = value.MapIndex(key)
			if !value.IsValid() {
				return nil, fmt.Errorf("map has no entry for key %v", index)
			}
		case reflect.Array, reflect.Slice, reflect.String:
			i, err := templateSequenceIndex(key, value.Len())
			if err != nil {
				return nil, err
			}
			value = value.Index(i)
		default:
			return nil, fmt.Errorf("cannot index %s", value.Type())
		}
	}
	if !value.IsValid() {
		return nil, fmt.Errorf("cannot index nil")
	}
	return value.Interface(), nil
}

func templateSequenceIndex(key reflect.Value, length int) (int, error) {
	var index int64
	switch key.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		index = key.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if key.Uint() >= uint64(length) {
			return 0, fmt.Errorf("index out of range")
		}
		index = int64(key.Uint())
	default:
		return 0, fmt.Errorf("sequence index must be an integer")
	}
	if index < 0 || index >= int64(length) {
		return 0, fmt.Errorf("index out of range: %d", index)
	}
	return int(index), nil
}
