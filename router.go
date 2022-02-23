package cuttle

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"io"
	"mime/multipart"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

var cutleContextType = reflect.TypeOf((*Context)(nil)).Elem()

type FinalResolver func(ctx Context) ([]reflect.Value, error)
type ResolverFunc func(ctx Context) (interface{}, error)
type MiddlewareFunc = echo.MiddlewareFunc
type ErrorHandlerFunc = func(err error, ctx Context)

type Context interface {
	echo.Context
}

type Cuttle struct {
	*echo.Echo
	ErrorHandler ErrorHandlerFunc
}

func New() *Cuttle {
	e := echo.New()
	e.HideBanner = true
	e.Logger.Info("[Cuttle 3:>] is based off Echo")
	return &Cuttle{
		e,
		nil,
	}
}

func (r *Cuttle) Method(method, path string, userHandler interface{}, middleware ...MiddlewareFunc) {
	finalResolver := r.handle(path, userHandler)

	r.Echo.Add(method, path, func(context echo.Context) error {
		in, err := finalResolver(Context(context))
		if err != nil {
			return fmt.Errorf("request validation failed: %w", err)
		}

		// assume that the validation failed
		if in == nil {
			return nil
		}

		retVal := reflect.ValueOf(userHandler).Call(in)
		if retVal[0].IsNil() {
			return nil
		}
		if r.ErrorHandler != nil {
			r.ErrorHandler(retVal[0].Interface().(error), context)
			return nil
		}
		return retVal[0].Interface().(error)
	}, middleware...)
}

func (r *Cuttle) GET(path string, userHandler interface{}, middleware ...MiddlewareFunc) {
	r.Method("GET", path, userHandler, middleware...)
}

func (r *Cuttle) POST(path string, userHandler interface{}, middleware ...MiddlewareFunc) {
	r.Method(http.MethodPost, path, userHandler, middleware...)
}

func (r *Cuttle) DELETE(path string, userHandler interface{}, middleware ...MiddlewareFunc) {
	r.Method(http.MethodDelete, path, userHandler, middleware...)
}

func (r *Cuttle) HEAD(path string, userHandler interface{}, middleware ...MiddlewareFunc) {
	r.Method(http.MethodHead, path, userHandler, middleware...)
}

func (r *Cuttle) PUT(path string, userHandler interface{}, middleware ...MiddlewareFunc) {
	r.Method(http.MethodPut, path, userHandler, middleware...)
}

func (r *Cuttle) OPTIONS(path string, userHandler interface{}, middleware ...MiddlewareFunc) {
	r.Method(http.MethodOptions, path, userHandler, middleware...)
}

func (r *Cuttle) CONNECT(path string, userHandler interface{}, middleware ...MiddlewareFunc) {
	r.Method(http.MethodConnect, path, userHandler, middleware...)
}

func (r *Cuttle) PATCH(path string, userHandler interface{}, middleware ...MiddlewareFunc) {
	r.Method(http.MethodPatch, path, userHandler, middleware...)
}

func (r *Cuttle) TRACE(path string, userHandler interface{}, middleware ...MiddlewareFunc) {
	r.Method(http.MethodTrace, path, userHandler, middleware...)
}

// handle returns a finalresolver function that returns the arguments passed to the userHandler as an array of reflect.Value
func (r *Cuttle) handle(path string, userHandler interface{}) FinalResolver {
	// validate userHandler
	handlerType := reflect.TypeOf(userHandler)
	if handlerType.Kind() != reflect.Func {
		panic(fmt.Sprintf("type of userHandler is '%v', not func", handlerType.Kind()))
	}
	if handlerType.NumIn() < 1 {
		panic(fmt.Sprintf("userHandler '%v' can only accept one or more argument", path))
	}

	if handlerType.NumOut() != 1 || handlerType.Out(0).String() != "error" {
		panic(fmt.Sprintf("userHandler '%v' should only return error", path))
	}
	// value to pass to function argument
	//   |			validation passed
	//   |           |     unhandled error
	var inputResolvers []func(ctx Context) (reflect.Value, bool, error)
	for i := 0; i < handlerType.NumIn(); i++ {
		var res func(ctx Context) (reflect.Value, bool, error)
		inType := handlerType.In(i)
		log.Debug("In Type String ", inType.String())

		type ValidationFail struct {
			Field string `json:"field"`
			Err   string `json:"error"`
		}

		if inType.Kind() == reflect.Struct && inType.NumField() > 0 {
			firstField := inType.Field(0)
			switch firstField.Type {
			case reflect.TypeOf(FromJson{}):
				log.Debug("[JSON] Assigned as json", inType)
				res = func(ctx Context) (reflect.Value, bool, error) {
					val := reflect.New(inType)
					return val.Elem(), true, json.NewDecoder(ctx.Request().Body).Decode(val.Interface())
				}
			case reflect.TypeOf(AsReturn{}):
				log.Debug("[Return] struct assigned as return type", inType)
				statusCode := firstField.Tag.Get("code")
				_ = statusCode // TODO: handle return values
				res = func(ctx Context) (reflect.Value, bool, error) {
					val := reflect.New(inType)
					return val.Elem(), true, nil
				}
			default:
				var resolvers = r.structResolvers(inType)

				// This gets called during the request
				res = func(context Context) (reflect.Value, bool, error) {
					in := reflect.New(inType)
					var failures []ValidationFail

					for i, resolver := range resolvers {
						if resolver == nil {
							continue
						}
						value, err := resolver(context)
						if err != nil {
							failures = append(failures, ValidationFail{
								Field: inType.Field(i).Name,
								Err:   err.Error(),
							})
							log.Debug("Validation failed", failures[len(failures)-1])
							continue
						}
						log.Debug("[DEBUG] Setting struct value", i, value)
						in.Elem().Field(i).Set(reflect.ValueOf(value))
					}

					if len(failures) != 0 {
						log.Debug("Validation failed for this request")
						return reflect.Value{}, false, context.JSON(http.StatusBadRequest, map[string]interface{}{
							"message": "validation failed",
							"fields":  failures,
						})
					}

					return in.Elem(), true, nil
				}
			}

			inputResolvers = append(inputResolvers, res)
			continue
		}

		// pass the usual echo context if it's just that
		if inType.Implements(cutleContextType) {
			// This gets called during the request
			res = func(context Context) (reflect.Value, bool, error) {
				return reflect.ValueOf(context), true, nil
			}
		}

		inputResolvers = append(inputResolvers, res)
	}

	var finalResolver FinalResolver = func(ctx Context) ([]reflect.Value, error) {
		var vals []reflect.Value
		for _, resolver := range inputResolvers {
			value, ok, err := resolver(ctx)
			if !ok {
				return nil, fmt.Errorf("request validation failed")
			}
			if err != nil {
				return nil, err
			}
			vals = append(vals, value)
		}
		return vals, nil
	}

	return finalResolver
}

// structResolvers only gets called on initialization of the handler, not during the request
func (r *Cuttle) structResolvers(inT reflect.Type) []ResolverFunc {
	var resolvers []ResolverFunc
	// iterate through struct fields
	for i := 0; i < inT.NumField(); i++ {
		var structResolver ResolverFunc = nil
		field := inT.Field(i)
		structTag := inT.Field(i).Tag

		getOption, tag := r.getTags(structTag, inT.Field(i).Name)
		log.Debug("[DEBUG] field info:", tag, structTag)

		var ctxResolvers ContextResolvers
		lookup, ok := structTag.Lookup("bind") // checks for > Field Type `bind:"query,param"`
		if ok {                                //						    ^^^^^^^^^^^^^^^^^
			ctxResolvers = GetResolvers(strings.Split(lookup, ",")...)
		} else {
			// default resolves if theres no specified bind
			ctxResolvers = GetResolvers("param", "query")
		}
		log.Debug("[DEBUG] resolvers:", tag, ctxResolvers)

		// skip unexported fields
		if !field.IsExported() {
			resolvers = append(resolvers, nil)
			continue
		}

		// handles unexported fields, nil as resolver to skip. Checks for > Field Type `return:"200"`
		// value will be the http status code
		value, ok := structTag.Lookup("return")
		if ok || !field.IsExported() {
			log.Debug("[DEBUG] field is unexported or return", field, ok, field.IsExported())
			resolvers = append(resolvers, nil)
			if !ok {
			}
			_ = value
			// TODO: handle return fields here
			continue
		}

		// handles coercion from webRequest to struct type
		switch field.Type.Kind() {
		case reflect.String:
			structResolver = func(ctx Context) (interface{}, error) {
				return ctxResolvers.Get(tag, getOption, ctx)
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			structResolver = func(ctx Context) (interface{}, error) {
				get, err := ctxResolvers.Get(tag, getOption, ctx)
				if err != nil {
					return nil, err
				}
				atoi, err := strconv.Atoi(get)
				if err != nil {
					return reflect.Value{}, fmt.Errorf("not a number: %w", err)
				}
				ret := reflect.New(field.Type)
				ret.Elem().SetInt(int64(atoi))
				return ret.Elem().Interface(), nil
			}
		case reflect.Float32, reflect.Float64:
			structResolver = func(ctx Context) (interface{}, error) {
				get, err := ctxResolvers.Get(tag, getOption, ctx)
				if err != nil {
					return nil, err
				}
				val, err := strconv.ParseFloat(get, 64)
				if err != nil {
					return reflect.Value{}, fmt.Errorf("not a number: %w", err)
				}
				ret := reflect.New(field.Type)
				ret.Elem().SetFloat(val)
				return ret.Elem().Interface(), nil
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			structResolver = func(ctx Context) (interface{}, error) {
				get, err := ctxResolvers.Get(tag, getOption, ctx)
				if err != nil {
					return nil, err
				}
				val, err := strconv.ParseUint(get, 10, 64)
				if err != nil {
					return reflect.Value{}, fmt.Errorf("not a number: %w", err)
				}
				ret := reflect.New(field.Type)
				ret.Elem().SetUint(val)
				return ret.Elem().Interface(), nil
			}
		case reflect.Interface:
			log.Debug("[DEBUG] Field", field.Type, "as interface", field.Type)

			// Check if its a file reader then pipe the field file writer
			if field.Type == reflect.TypeOf((*io.Reader)(nil)).Elem() {
				log.Debug("[DEBUG] assigning field as body:", field.Name)
				structResolver = func(ctx Context) (interface{}, error) {
					return bufio.NewReader(ctx.Request().Body), nil
				}
			}
		// checks for pointer struct fields
		case reflect.Ptr:
			// if a multipart fileheader then return the form's fileheader
			if field.Type.AssignableTo(reflect.TypeOf((*multipart.FileHeader)(nil))) {
				log.Debug("[DEBUG] assigning field as file header:", field.Name)
				structResolver = func(ctx Context) (interface{}, error) {
					return ctx.FormFile(tag)
				}
			}
		}

		// Set as echo.context here
		if field.Type.ConvertibleTo(cutleContextType) {
			log.Debug("[DEBUG] Field", field.Type, "implements", cutleContextType)
			structResolver = func(ctx Context) (interface{}, error) {
				return ctx, nil
			}
		}

		resolvers = append(resolvers, structResolver)
	}

	return resolvers
}

func (r *Cuttle) getTags(structTag reflect.StructTag, tag string) (CSRGetOption, string) {
	getOption := CSRGetOption{
		Sensitive: false,
		Required:  false,
	}
	asTag, ok := structTag.Lookup("as")
	if ok {
		log.Debug("[DEBUG] as specified", tag, "->", asTag)
		i := strings.Split(asTag, ",")
		if i[0] != "" {
			tag = i[0]
		}
		if len(i) > 1 {
			for _, v := range i[1:] {
				switch v {
				case "sensitive":
					getOption.Sensitive = true
				case "required":
					getOption.Required = true
				}
			}
		}
	}
	return getOption, tag
}
