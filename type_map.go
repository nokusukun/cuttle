package locust

import (
	"reflect"
	"strings"
)

type GenerateTypeMapOptions struct {
	IncludeUnexported bool
	LookupTags []string

}

// GenerateTypeMap generates a map[string]interface where it lays out the name:type of the struct fields
func GenerateTypeMap(typeVal reflect.Type) map[string]interface{} {
	typeMap := map[string]interface{}{}
	for i := 0; i < typeVal.NumField(); i++ {
		value := typeVal.Field(i)
		if !value.IsExported() {
			continue
		}

		name := value.Name
		if lookup, ok := value.Tag.Lookup("json"); ok {
			name = strings.Split(lookup, ",")[0]
		}

		var t interface{} = value.Type.Kind().String()
		switch value.Type.Kind() {
		case reflect.Struct:
			t = GenerateTypeMap(value.Type)
		case reflect.Slice:
			t = []interface{}{GenerateTypeMap(value.Type.Elem())}
		}

		typeMap[name] = t
	}
	return typeMap
}

