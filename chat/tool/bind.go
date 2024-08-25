package tool

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/swdunlop/ollama-client/chat/protocol"
)

func (t *tool) bind(fn any) error {
	fv := reflect.ValueOf(fn)
	if fv.Kind() != reflect.Func {
		return fmt.Errorf(`cannot bind %T as a tool`, fn)
	}
	t.fn = fv

	spec := t.spec
	spec.Type = `function`
	if spec.Function == nil {
		spec.Function = new(protocol.ToolFunction)
	}
	if spec.Function.Name == `` {
		t.bindFunctionName(fv)
	}

	ft := fv.Type()

	switch ft.NumIn() {
	case 1:
		t.inputType = ft.In(0)
	case 2:
		if ft.In(0).Implements(contextInterface) {
			t.expectsContext = true
		}
		t.inputType = ft.In(1)
	default:
		return fmt.Errorf(`incorrect input parameters for tool %q`, spec.Function.Name)
	}

	switch ft.NumOut() {
	case 1:
		// That's fine; we assume this is the content output
	case 2:
		if !ft.Out(1).Implements(errorInterface) {
			return fmt.Errorf(`incorrect output parameters for tool %q`, spec.Function.Name)
		}
		t.returnsErrors = true
	default:
		return fmt.Errorf(`incorrect output parameters for tool %q`, spec.Function.Name)
	}
	t.contentType = ft.Out(0)

	if t.inputType.Kind() != reflect.Struct {
		return fmt.Errorf(`incorrect input parameter for tool %q; got %T, but a structure is required`,
			spec.Function.Name,
			t.inputType.String(),
		)
	}

	return t.bindInputParameters(t.inputType)
}

func (t *tool) bindFunctionName(fv reflect.Value) {
	// Some function names:
	//  - main.main -- private function in package main.
	//  - reflect.ValueOf -- function exported by reflect.
	//  - net/http.ListenAndServe -- function exported by net/http.
	//  - main.main.func1 -- function inside function main.
	//  - main.init.func1 -- function defined as a var of the main package

	name := runtime.FuncForPC(fv.Pointer()).Name()
	if lastSlash := strings.LastIndexByte(name, '/'); lastSlash >= 0 {
		name = name[lastSlash+1:]
	}
	if firstDot := strings.IndexByte(name, '.'); firstDot >= 0 {
		name = name[firstDot+1:]
	}
	t.spec.Function.Name = name
}

func (t *tool) bindInputParameters(it reflect.Type) error {
	n := it.NumField()
	for i := 0; i < n; i++ {
		fs := it.Field(i)
		if !fs.IsExported() {
			continue
		}
		if fs.Anonymous {
			t.bindInputParameters(fs.Type)
			continue
		}

		name := fs.Name
		if json, ok := fs.Tag.Lookup(`json`); ok {
			name = strings.SplitN(json, `,`, 2)[0]
		}
		if name == `` {
			continue // ignore explicitly anonymous fields.
		}

		use := fs.Tag.Get(`use`)
		jsonType := fs.Tag.Get(`type`)
		if jsonType == `` {
			switch fs.Type.Kind() {
			case reflect.Array:
				jsonType = `array` // TODO: of... ?
			case reflect.Struct:
				jsonType = `object`
			case reflect.Map:
				jsonType = `object` // TODO: of.., ?
			case reflect.Int, reflect.Uint,
				reflect.Int8, reflect.Uint8,
				reflect.Int16, reflect.Uint16,
				reflect.Int32, reflect.Uint32,
				reflect.Int64, reflect.Uint64:
				jsonType = `number`
			case reflect.Bool:
				jsonType = `bool`
			case reflect.String:
				jsonType = `string`
			}
		}
		t.updateProperty(name, func(fp protocol.ToolFunctionProperty) protocol.ToolFunctionProperty {
			if use != `` {
				fp.Description = use
			}
			if fp.Type == `` {
				fp.Type = jsonType
			}
			return fp
		})
	}
	return nil // TODO
}

var (
	contextInterface = reflect.TypeOf((*context.Context)(nil)).Elem()
	errorInterface   = reflect.TypeOf((*error)(nil)).Elem()
)

// wrongOutputs = fmt.Errorf(`tool functions must return content and may return an error`)
// wrongInputs  = fmt.Errorf(`tool functions should accept a context and/or a structure in that order`)
