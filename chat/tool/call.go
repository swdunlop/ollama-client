package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
)

func (t *tool) Call(ctx context.Context, parameters json.RawMessage) (json.RawMessage, error) {
	q := reflect.New(t.inputType).Elem()
	err := json.Unmarshal(parameters, q.Addr().Interface())
	if err != nil {
		return nil, fmt.Errorf(`%w while parsing parameters for %q`, err, t.spec.Function.Name)
	}
	var ret []reflect.Value
	if t.expectsContext {
		ret = t.fn.Call([]reflect.Value{
			reflect.ValueOf(ctx),
			q,
		})
	} else {
		ret = t.fn.Call([]reflect.Value{q})
	}

	if t.returnsErrors {
		if err, ok := ret[1].Interface().(error); ok {
			return nil, err
		}
	}

	js, err := json.Marshal(ret[0].Interface())
	if err != nil {
		return nil, fmt.Errorf(`%w while formatting content for %q`, err, t.spec.Function.Name)
	}

	return js, nil
}
