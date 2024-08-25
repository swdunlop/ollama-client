package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestBind(t *testing.T) {
	t.Run(`Anonymous`, func(t *testing.T) {
		t.Run(`NoContext`, func(t *testing.T) {
			noContextNoInput := func() {}                           // fail
			noContextNoOutput := func(in struct{}) {}               // fail
			noContextNoError := func(in struct{}) int { return 42 } // succ
			testBind(t, `NoInput`, noContextNoInput, func(t *testing.T, tool *tool, err error) {
				if err == nil {
					t.Error(`expected error since there were no input parameters`)
				}
			})
			testBind(t, `NoOutput`, noContextNoOutput, func(t *testing.T, tool *tool, err error) {
				if err == nil {
					t.Error(`expected error since there were no output parameters`)
				}
			})
			testBind(t, `NoError`, noContextNoError, func(t *testing.T, tool *tool, err error) {
				if err != nil {
					t.Fatal(`no error was expected`)
				}
				if tool.returnsErrors == true {
					t.Error(`expected no error return`)
				}
				if tool.contentType.Kind() != reflect.Int {
					t.Error(`expected integer content`)
				}
			})
		})
		simple := func(in struct {
			A int
			B int
		}) int {
			return in.A + in.B
		} // succ
		testBind(t, `Simple`, simple, func(t *testing.T, tool *tool, err error) {
			if err != nil {
				t.Fatal(`no error was expected`)
			}
			if tool.returnsErrors == true {
				t.Error(`expected error return`)
			}
			if tool.expectsContext == true {
				t.Error(`expected no context expectation`)
			}
			if tool.contentType.Kind() != reflect.Int {
				t.Error(`expected integer content`)
			}
		})
		full := func(ctx context.Context, in struct {
			A int
			B int
		}) (int, error) {
			return in.A + in.B, fmt.Errorf(`post no bills`)
		} // succ
		testBind(t, `Full`, full, func(t *testing.T, tool *tool, err error) {
			if err != nil {
				t.Fatal(`no error was expected`)
			}
			if tool.expectsContext != true {
				t.Error(`expected context expectation`)
			}
			if tool.returnsErrors != true {
				t.Error(`expected error return`)
			}
			if tool.contentType.Kind() != reflect.Int {
				t.Error(`expected integer content`)
			}
		})
	})

	testBind(t, `Simple`, simple, func(t *testing.T, tool *tool, err error) {
		if err != nil {
			t.Fatal(`no error was expected`)
		}
		if tool.returnsErrors == true {
			t.Error(`expected no error return`)
		}
		if tool.expectsContext == true {
			t.Error(`expected no context expectation`)
		}
		if tool.contentType.Kind() != reflect.String {
			t.Error(`expected string content`)
		}
	})
	testBind(t, `Full`, full, func(t *testing.T, tool *tool, err error) {
		if err != nil {
			t.Fatal(`no error was expected`)
		}
		if tool.expectsContext != true {
			t.Error(`expected context expectation`)
		}
		if tool.returnsErrors != true {
			t.Error(`expected error return`)
		}
		if tool.contentType.Kind() != reflect.String {
			t.Error(`expected string content`)
		}
	})
	testBind(t, `Complex`, findOrders, func(t *testing.T, tool *tool, err error) {
		if err != nil {
			t.Fatal(`no error was expected`)
		}
		if tool.expectsContext != true {
			t.Error(`expected context expectation`)
		}
		if tool.returnsErrors != true {
			t.Error(`expected error return`)
		}
		if tool.contentType.Kind() != reflect.Slice {
			t.Error(`expected slice content`)
		}
	})
}

func simple(q struct {
	A string
	B string
}) string {
	return q.A + q.B
}

func full(ctx context.Context, q struct {
	A string
	B string
}) (string, error) {
	return q.A + q.B, nil
}

func findOrders(ctx context.Context, q struct {
	Customer Optional[id]        `json:"customerID" use:"Customer ID; only orders from this customer will be found."`
	Name     Optional[string]    `json:"name"       use:"Order Name; only orders whose name contain this string will be found."`
	Start    Optional[time.Time] `json:"start"      use:"Start Time in ISO-8601; only orders created on or after this time will be found."`
	End      Optional[time.Time] `json:"end"        use:"End Time in ISO-8601; only orders created on or before this time will be found."`
}) ([]order, error) {
	panic(`TODO`)
}

type id uint64
type order struct{}

func testBind(t *testing.T, name string, toolFn any, fn func(t *testing.T, tool *tool, err error)) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		test, _ := New()
		if test == nil {
			t.Fatalf(`New should return a an empty tool, even though it shoudl fail`)
		}
		t.Helper()
		tool := test.(*tool)
		err := tool.bind(toolFn)
		t.Log(`spec`, fmtJSON(tool.spec))
		t.Log(`err`, err)
		fn(t, tool, err)
	})
}

func fmtJSON(v any) string {
	js, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(js)
}
