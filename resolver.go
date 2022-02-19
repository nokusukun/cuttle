package locust

import (
	"fmt"
	"reflect"
	"strings"
)

type FromJson struct {}
type AsReturn struct {}

func GetNameTag(a interface{}, index int) (string, reflect.StructTag) {
	val := reflect.Indirect(reflect.ValueOf(a))
	return val.Type().Field(index).Name, val.Type().Field(index).Tag
}

type ContextResolvers []ContextResolverFunc
type ContextResolverFunc func(string, Context) string
const ResolveAsFile = "locust.resolveasfile"
type CSRGetOption struct {
	Sensitive bool
	Required bool
}

var ErrNoValueOnRequiredField = fmt.Errorf("no value found on required field")

func (cr ContextResolvers) Get(name string, option CSRGetOption, ctx Context) (string, error) {
	var err error
	if option.Required {
		err = ErrNoValueOnRequiredField
	}
	for _, resolverFunc := range cr {
		s := resolverFunc(name, ctx)
		if s == "" && !option.Sensitive {
			s = resolverFunc(strings.ToLower(name), ctx)
		}
		if s != "" {
			return s, nil
		}
	}
	return "", err
}

var ctxResolvers = map[string]ContextResolverFunc{
	"query": func(name string, ctx Context) string {
		x := ctx.QueryParam(name)
		return x
	},
	"param": func(name string, ctx Context) string {
		x := ctx.Param(name)
		return x
	},
	"header": func (name string, ctx Context) string {
		x := ctx.Request().Header.Get(name)
		return x
	},
	"form": func(name string, ctx Context) string {
		x := ctx.Request().FormValue(name)
		return x
	},
	"file": func(name string, ctx Context) string {
		return ResolveAsFile
	},
}

func GetResolvers(solvers ...string) ContextResolvers {
	var cr ContextResolvers
	for _, solver := range solvers {
		if r, ok := ctxResolvers[solver]; ok {
			cr = append(cr, r)
		}
	}

	return cr
}
