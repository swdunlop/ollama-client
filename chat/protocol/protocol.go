// Package protocol describes the structure of chat requests and responses for the Ollama API.
package protocol

import (
	"encoding/json"
	"time"
)

// Request describes the structure of a chat request.  It is not generally necessary to construct this yourself,
// instead, use the various options provided.
type Request struct {
	// Model is the model name; this is required by Ollama.
	//
	// See https://github.com/ollama/ollama/blob/main/docs/api.md#model-names
	Model string `json:"model"`

	// Messages is a list of messages.
	Messages []Message `json:"messages,omitempty"`

	// Tools is a list of tools available to the model.  This may not be combined with streaming in Ollama
	// as of 2024-08-24.
	Tools []Tool `json:"tools,omitempty"`

	// Format, if present, should be "json" to indicate that the content of the messages in the response
	// should be JSON.
	Format string `json:"format,omitempty"`

	// Options is a map of model parameter overrides, such as temperature.
	//
	// See https://github.com/ollama/ollama/blob/main/docs/modelfile.md#valid-parameters-and-values
	Options map[string]any `json:"options,omitempty"`

	// KeepAlive, if present, should be a Go duration string, such as "5m", indicating how long the model
	// should stay in memory after the request.
	KeepAlive string `json:"keep_alive,omitempty"`

	// Stream tells the client to stream the response incrementally.
	Stream bool `json:"stream"`
}

// A Message contains a single message sent either from the client to the model or from the model to the client.
type Message struct {
	Role      Role       `json:"role"`
	Content   string     `json:"content"`
	Images    []Image    `json:"images"`
	ToolCalls []ToolCall `json:"tool_calls"`
}

func (*Request) OllamaAPI() (string, string)   { return `POST`, `/api/chat` }
func (*Request) OllamaResponse(status int) any { return new(Response) }

// Response describes the response from a chat request.
type Response struct {
	Model              string      `json:"model"`
	CreatedAt          time.Time   `json:"created_at"`
	Message            Message     `json:"message"`
	Done               bool        `json:"done"`
	TotalDuration      json.Number `json:"total_duration"`
	LoadDuration       json.Number `json:"load_duration"`
	PromptEvalCount    json.Number `json:"prompt_eval_count"`
	PromptEvalDuration json.Number `json:"prompt_eval_duration"`
	EvalCount          json.Number `json:"eval_count"`
	EvalDuration       json.Number `json:"eval_duration"`
}

// Image is a PNG encoded image.  This can be sent to multi-modal models like "llava" and "bakllava."
type Image []byte

// Role is the role of the message, such as `system`, `user`, `assistant`, or `tool`.
type Role string

const (
	SYSTEM    = Role(`system`)
	USER      = Role(`user`)
	ASSISTANT = Role(`assistant`)
	TOOL      = Role(`tool`)
)

// Tool describes a tool usable by the model.  Ollama only supports function tools, as of 2024-08-24.
type Tool struct {
	// Type must be present as "function"
	Type string `json:"type"`

	// Function describes a function usable as a tool by the model.
	Function *ToolFunction `json:"function,omitempty"`

	// This is not well documented in api.md yet -- the source of this structure is https://github.com/ollama/ollama/blob/main/api/types.go
}

// ToolFunction describes a function usable as a Tool.
type ToolFunction struct {
	// Name is the unique name of the tool.
	Name string `json:"name,omitempty"`

	// Description explains what the tool does to the model.
	Description string `json:"description,omitempty"`

	// Parameters describes the parameters accepted by the tool.
	Parameters struct {
		// Type describes the type of parameters.  This must be "object" for Ollama, as of 2024-08-24.
		Type string `json:"type,omitempty"`

		// Required lists properties that are required to be present.
		Required []string `json:"required,omitempty"`

		// Properties is a map of property names to their type and description.
		Properties map[string]ToolFunctionProperty `json:"properties,omitempty"`
	} `json:"parameters"`

	// This is not well documented in api.md yet -- the source of this structure is https://github.com/ollama/ollama/blob/main/api/types.go
}

// A ToolFunctionProperty describes one of the properties found in a map of tool function properties.
type ToolFunctionProperty struct {
	// Type is the type of the property.  This is interpreted by models in various ways.
	Type string `json:"type"`

	// Description is an explanation of the property for the model.
	Description string `json:"description"`

	// Enum is a list of acceptable values for properties that are enumerated.
	Enum []string `json:"enum,omitempty"`
}

// ToolCall describes a call by the model of a function that should have been described as available as a tool.
type ToolCall struct {
	// Function is the function call.  Ollama only supports calling functions, as of 2024-08-24, regardless of
	// whatever the model supports.
	Function *ToolCallFunction `json:"function"`
}

// ToolCallFunction describes a function call.
type ToolCallFunction struct {
	// Name is the name of the tool, and should match the name from ToolFunction
	Name string `json:"name"`

	// Arguments contains the parameters from the model.
	Arguments json.RawMessage `json:"arguments"`
}

// https://github.com/ollama/ollama/blob/main/docs/api.md#generate-a-chat-completion
