// Package tool can be used to define a tool usable by tool capable Ollama models.
//
// # Example
//
//		func findOrders(ctx context.Context, q struct {
//		  Customer  tool.Optional[ID]        `json:"customerID" use:"Customer ID; only orders from this customer will be found."`
//		  Name      tool.Optional[string]    `json:"name"       use:"Order Name; only orders whose name contain this string will be found."`
//		  Start     tool.Optional[time.Time] `json:"start"      use:"Start Time in ISO-8601; only orders created on or after this time will be found."`
//		  End       tool.Optional[time.Time] `json:"end"        use:"End Time in ISO-8601; only orders created on or before this time will be found."`
//		}) ([]Order, error) { panic(`TODO`) }
//
//	 findOrdersTool = tool.New(
//	   tool.Func(findOrders),
//	   tool.Description(`finds tools`)
package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/iancoleman/strcase"
	"github.com/swdunlop/ollama-client/chat/protocol"
)

// New constructs a new tool with the specified options.
func New(options ...Option) (Interface, error) {
	t := new(tool)
	t.spec.Type = `function`
	t.spec.Function = new(protocol.ToolFunction)
	t.spec.Function.Parameters.Type = `object`
	t.spec.Function.Parameters.Properties = make(map[string]protocol.ToolFunctionProperty, 16)
	for _, option := range options {
		option(t)
		if t.err != nil {
			return nil, t.err
		}
	}

	for _, fixup := range t.fixups {
		fixup(t)
		if t.err != nil {
			return nil, t.err
		}
	}
	t.fixups = nil

	return t, t.validate()
}

// Func specifies this is a tool function and associates it with a Go function.  This will set the name of the tool,
// if it is not already set using Name.  The function must take a context as its first input, and a structure as its
// second input, and should return a value and an error output.
//
// The public fields from that structure will be bound as parameters accepted by the tool, using the "name" and "use"
// struct tags for the name of the parameter and its description.
func Func(fn any) Option {
	return func(t *tool) {
		t.err = t.bind(fn)
	}
}

// Name provides a name for the tool.  Without this option, the name is inferred from the Go function name.
func Name(name string) Option {
	return func(t *tool) {
		t.spec.Function.Name = name
	}
}

// Description provides a description for the tool.  This is essential for a model to understand the purpose of the
// tool.
func Description(description string) Option {
	return func(t *tool) {
		t.spec.Function.Description = description
	}
}

// Enum adds allowable values for the named parameter.
func Enum(parameter string, values ...string) Option {
	return propertyOption(parameter, func(p protocol.ToolFunctionProperty) protocol.ToolFunctionProperty {
		p.Enum = append(p.Enum, values...)
		return p
	})
}

// Parameter declares a parameter for the tool.
func Parameter(parameter, parameterType, description string) Option {
	return propertyOption(parameter, func(p protocol.ToolFunctionProperty) protocol.ToolFunctionProperty {
		p.Description = description
		p.Type = parameterType
		return p
	})
}

// CamelNames converts all parameter names to camel case after the Func and Parameter options resolve using `strcase.ToLowerCamel`.
func CamelNames() Option {
	return FixParameterNames(strcase.ToLowerCamel)
}

// FixParameterNames fixes all parameter names by applying the provided function to rename them.  If the new name is an empty string,
// the parameter is deleted from the specification.  This is useful with a function like `strcase.ToLowerCamel` when relying on reflection
// to map the parameters from a structure, since exported fields are TitleCase in Go, which is atypical for function parameters.
//
// This is a fixup, and is applied after all other non-fixup options, like Func and Parameter.
func FixParameterNames(fix func(string) string) Option {
	return fixupOption(func(t *tool) {
		prev := t.spec.Function.Parameters.Properties
		next := make(map[string]protocol.ToolFunctionProperty, len(prev))
		for name, spec := range prev {
			name = fix(name)
			if name == `` {
				continue
			}
			next[name] = spec
		}
		t.spec.Function.Parameters.Properties = next
		n := 0
		for _, name := range t.spec.Function.Parameters.Required {
			name = fix(name)
			if name == `` {
				continue
			}
			t.spec.Function.Parameters.Required[n] = name
			n++
		}
		t.spec.Function.Parameters.Required = t.spec.Function.Parameters.Required[:n]
	})
}

func fixupOption(fixup Option) Option {
	return func(t *tool) { t.fixups = append(t.fixups, fixup) }
}

// propertyOption constructs a new tool option that affects the named parameter; this will ensure the parameter exists
// and will replace it with an improved version each time it is applied.
func propertyOption(
	parameter string, fn func(protocol.ToolFunctionProperty) protocol.ToolFunctionProperty,
) Option {
	return func(t *tool) { t.updateProperty(parameter, fn) }
}

func (t *tool) updateProperty(
	parameter string, fn func(protocol.ToolFunctionProperty) protocol.ToolFunctionProperty,
) {
	p := t.spec.Function.Parameters.Properties[parameter]
	p = fn(p)
	t.spec.Function.Parameters.Properties[parameter] = p
}

// Required marks that the named parameters are required.
func Required(parameters ...string) Option {
	return func(t *tool) {
		t.spec.Function.Parameters.Required = append(t.spec.Function.Parameters.Required, parameters...)
	}
}

// An Option affects how a tool is defined.
type Option func(*tool)

// Interface describes the interface provided by tools defined by this package.
type Interface interface {
	// Call will call the tool, passing the provided parameters and context, then returning the content.
	Call(ctx context.Context, parameters json.RawMessage) (json.RawMessage, error)

	// Tool returns a tool description suitable for sending to Ollama.
	Tool() protocol.Tool
}

type tool struct {
	spec protocol.Tool
	fn   reflect.Value

	inputType      reflect.Type
	contentType    reflect.Type
	expectsContext bool
	returnsErrors  bool

	fixups []Option
	err    error
}

func (t *tool) Tool() protocol.Tool { return t.spec }

func (t *tool) validate() error {
	if err := t.validateDescription(); err != nil {
		return err
	}
	if err := t.validateParameters(); err != nil {
		return err
	}
	if err := t.validateRequired(); err != nil {
		return err
	}
	return nil
}

func (t *tool) validateDescription() error {
	if t.spec.Function.Description == `` {
		return fmt.Errorf(`function tools should have a description`)
	}
	return nil
}

func (t *tool) validateParameters() error {
	for name, parameter := range t.spec.Function.Parameters.Properties {
		if name == `` {
			return fmt.Errorf(`all parameters must have names`)
		}
		err := t.validateParameter(name, parameter)
		if err != nil {
			return fmt.Errorf(`%w while validating parameter %q`, err, name)
		}
	}
	return nil
}

func (t *tool) validateParameter(name string, parameter protocol.ToolFunctionProperty) error {
	if parameter.Type == `` {
		return fmt.Errorf(`missing parameter type`)
	}
	if parameter.Description == `` {
		return fmt.Errorf(`missing parameter description`)
	}
	return nil
}

func (t *tool) validateRequired() error {
	for _, name := range t.spec.Function.Parameters.Required {
		_, ok := t.spec.Function.Parameters.Properties[name]
		if !ok {
			return fmt.Errorf(`missing required parameter %q`, name)
		}
	}
	return nil
}

// # Input Parameters:
//
// A tool may have 0-2 input parameters, consisting of an optional context.Context followed by an optional
// Go structure.  Each field of the structure will be exposed as parameters of the tool, using the "tool"
// structure tag to denote the name, or the "json" tag if the tool tag is not present.
//
//   - If a field type implements the Enumerated interface, the property will include the Enum() values.
//
//   - All properties with names are required unless they are wrapped as Optional.
//
// # Arguments:
//
//   - context.Context -- this will not be added to the tool description but CallCtx will furnish this context.
//
//   - numeric types -- all of these will be given the type of "number"; json.Number is acceptable, but complex
//     numbers are not.
//
//   - bool / string -- this is fine as well
//
//   - Optional[T] -- the argument will be an optional parameter.
//
// # Example
//
//	findOrders := (ctx context.Context, q struct {
//	  Customer  tool.Optional[ID]        `tool:"customerID"`
//	  Name      tool.Optional[string]    `tool:"name"`
//	  Start     tool.Optional[time.Time] `tool:"start"`
//	  End       tool.Optional[time.Time] `tool:"end"`
//	}) ([]Order, error) { panic(`TODO`) }
//
//	content, err := tool.CallCtx(ctx, toolCall.Function.Arguments)
