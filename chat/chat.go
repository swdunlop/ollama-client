// Package chat details how to create a chat request for the Ollama API and how to process its response.
package chat

import (
	"context"

	"github.com/swdunlop/ollama-client/chat/message"
	"github.com/swdunlop/ollama-client/chat/protocol"
	"github.com/swdunlop/ollama-client/chat/tool"
	"github.com/swdunlop/ollama-client/chat/toolkit"
)

// Model specifies the model name; this is required by Ollama.
//
// See https://github.com/ollama/ollama/blob/main/docs/api.md#model-names
func Model(model string) Option { return func(q *Request) { q.Model = model } }

// System adds a message with the system role to the request.  This is useful for giving instructions to the model that have a higher
// priority than that of the user.
func System(content string, options ...message.Option) Option {
	return Message(protocol.SYSTEM, content, options...)
}

// Assistant adds a message with the assistant role to the request.  This role is the voice of the model.
func Assistant(content string, options ...message.Option) Option {
	return Message(protocol.ASSISTANT, content, options...)
}

// User adds a message with the user role to the request.  This gives more instructions to a model, but with a lower priority -- models
// are expected to treat messages with the system role as the absolute truth and when the user conflicts with the system, it should
// favor the system.
//
// Note the use of the weasel word "should" -- no model is perfect at this.
func User(content string, options ...message.Option) Option {
	return Message(protocol.USER, content, options...)
}

// Message adds a message to the request.
func Message(role Role, content string, options ...message.Option) Option {
	return func(q *Request) {
		m := protocol.Message{Role: role, Content: content}
		for _, option := range options {
			option(&m)
		}
		q.Messages = append(q.Messages, m)
	}
}

// Toolkit adds a set of tools that the model may call.  If used, the model may call tools as desired and the
// conversation will conclude only once the model has stopped calling tools.
//
// This is convenient, but for manual control, you may want to use the toolkit package directly or the tool package.
func Toolkit(tools ...Tool) Option {
	return func(r *Request) {
		for _, tool := range tools {
			r.Tools = append(r.Tools, tool.Tool())
		}
		tk := toolkit.New(tools...)
		// TODO: move this to toolkit.Hook ?
		r.hook(func(ctx context.Context, messages ...protocol.Message) ([]protocol.Message, error) {
			if len(messages) == 0 {
				return messages, nil // weird, but I'll allow it
			}
			last := messages[len(messages)-1]
			for _, call := range last.ToolCalls {
				ret, err := tk.Call(ctx, call)
				if err != nil {
					// TODO: does it make more sense to return these errors? should we gather all the errors? what do users expect?
					return nil, err
				}
				messages = append(messages, ret)
			}
			if len(last.ToolCalls) > 0 {
				return messages, Continue{}
			}
			return messages, nil
		})
	}
}

// Tools adds tools that the model may call, but will not handle the calls directly.  This is an alternative to the more
// convenient but less controllable Toolkit.
func Tools(tools ...Tool) Option {
	return func(r *Request) {
		for _, tool := range tools {
			r.Tools = append(r.Tools, tool.Tool())
		}
	}
}

// Tool is an alias to the tool interface.
type Tool = tool.Interface

// Hook adds a function that is called after a response is received to react when a new message is returned by the model.  This function
// can rewrite the messages and ask for the client to send the request again with the new set of messages by returning the special
// Continue error.  This is used by Toolkit to automatically call tools as desired by the model.
func Hook(hook func(ctx context.Context, messages ...protocol.Message) ([]protocol.Message, error)) Option {
	return func(r *Request) { r.hook(hook) }
}

// Temperature affects how random the response may be.  A 0.0 temperature should effectively avoid any deviation from the most probable
// response.  A 1.0 temperature affords some variation in responses.
func Temperature(temperature float64) Option {
	return requestOption(`temperature`, temperature)
}

func requestOption(name string, value any) Option {
	return func(r *Request) {
		if r.Options == nil {
			r.Options = make(map[string]any)
		}
		r.Options[name] = value
	}
}

// An Option affects the construction of a chat request.
type Option func(*Request)

type Role = protocol.Role

// Request describes the structure of a chat request.  It is not generally necessary to construct this yourself,
// instead, use the various options provided.
type Request struct {
	protocol.Request

	hooks []func(ctx context.Context, messages ...protocol.Message) ([]protocol.Message, error)
}

func (r *Request) hook(hook func(ctx context.Context, messages ...protocol.Message) ([]protocol.Message, error)) {
	r.hooks = append(r.hooks, hook)
}

// Request describes the structure of a chat request.  It is not generally necessary to construct this yourself,
// instead, use the various options provided.
type Response = protocol.Response

// https://github.com/ollama/ollama/blob/main/docs/api.md#generate-a-chat-completion

// Continue may be returned by a hook to tell the client to repeat the request after applying hooks.
type Continue struct{}

func (Continue) Error() string { return `please continue` }
