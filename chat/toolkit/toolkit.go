// Package toolkit wraps a set of tools into a unified interface that can be easily used to process tool calls.
package toolkit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/swdunlop/ollama-client/chat/protocol"
	"github.com/swdunlop/ollama-client/chat/tool"
)

// New constructs a new toolkit from the provided tools.
func New(tools ...Tool) Interface {
	tk := new(toolkit)
	tk.list = append([]Tool(nil), tools...)
	tk.table = make(map[string]tool.Interface, len(tools))
	for _, tool := range tools {
		// TODO: nag about duplicates?
		tk.table[tool.Tool().Function.Name] = tool
	}
	return tk
}

type toolkit struct {
	list  []Tool
	table map[string]Tool
}

// Call calls a tool from the toolkit.
func (tk *toolkit) Call(ctx context.Context, call protocol.ToolCall) (ret protocol.Message, err error) {
	ret.Role = protocol.TOOL
	defer func() {
		if err != nil {
			msg := struct {
				Error string `json:"error"`
			}{Error: err.Error()}
			js, _ := json.Marshal(msg)
			ret.Content = string(js)
		}
	}()
	if call.Function == nil {
		err = fmt.Errorf(`only tool function calls are supported`)
		return
	}
	tool := tk.table[call.Function.Name]
	if tool == nil {
		err = fmt.Errorf(`tool %q not found`, call.Function.Name)
		return
	}
	content, err := tool.Call(ctx, call.Function.Arguments)
	if err != nil {
		return
	}
	ret.Content = string(content)
	return
}

func (tk *toolkit) Tools() []Tool {
	return append([]Tool(nil), tk.list...)
}

// Interface describes the toolkit interface.
type Interface interface {
	// Call will call the requested tool, if it exists.  It will return an error if the tool did not exist, or if
	// the tool itself returned an error.
	Call(ctx context.Context, call protocol.ToolCall) (protocol.Message, error)

	// Tools returns a list of the tools supported by the toolkit.
	Tools() []Tool
}

type Tool = tool.Interface
