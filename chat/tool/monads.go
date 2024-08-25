package tool

import (
	"encoding/json"
)

// None returns an optional value where the value is absent.
func None[T any]() Optional[T] { return Optional[T]{present: false} }

// Some returns an optional value where the value is present.
func Some[T any](value T) Optional[T] { return Optional[T]{true, value} }

// Optional is a generic that can be used by tools to indicate that a value is optional.
type Optional[T any] struct {
	present bool
	value   T
}

func (opt Optional[T]) Present() bool { return opt.present }
func (opt Optional[T]) Absent() bool  { return !opt.present }
func (opt Optional[T]) Value() T      { return opt.value }

func (opt *Optional[T]) UnmarshalJSON(js []byte) error {
	var value T
	err := json.Unmarshal(js, &value)
	if err != nil {
		return err
	}
	*opt = Some[T](value)
	return nil
}

func (opt Optional[T]) MarshalJSON() ([]byte, error) {
	if opt.present {
		return json.Marshal(opt.value)
	}
	return json.Marshal(nil)
}
