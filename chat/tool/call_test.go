package tool

import (
	"context"
	"encoding/json"
	"testing"
)

func TestCall(t *testing.T) {
	tool, err := New(Func(hello), Description("says hello to someone"))
	if err != nil {
		t.Fatalf(`hello should be a valid tool; got %v`, err)
	}
	ret, err := tool.Call(context.Background(), json.RawMessage(`{"name": "world"}`))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(`ret`, string(ret))
	if string(ret) != `{"hello":"world"}` {
		t.Fatalf(`expected {"hello":"world"}`)
	}
}

func hello( /* ctx context.Context, */ q struct {
	Name string `json:"name" use:"who should we say hello to?"`
}) (r struct {
	Hello string `json:"hello"`
}, err error) {
	r.Hello = q.Name
	return
}
