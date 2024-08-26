package embed

import "time"

// Model specifies the model name; this is required by Ollama.
//
// See https://github.com/ollama/ollama/blob/main/docs/api.md#model-names
func Model(model string) Option { return func(q *Request) { q.Model = model } }

// Temperature affects how random the response may be.  A 0.0 temperature should effectively avoid any deviation from the most probable
// response.  A 1.0 temperature affords some variation in responses.
func Temperature(temperature float64) Option {
	return requestOption(`temperature`, temperature)
}

// Input appends one or more inputs to the request.
func Input(inputs ...string) Option {
	return func(r *Request) { r.Input = append(r.Input, inputs...) }
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

type Request struct {
	// Model identifies the ollama model name, such as nomic-embed-text:latest
	Model string `json:"model"`

	// Input is a list of strings.
	Input []string `json:"input"`

	// Truncate is true if Ollama should truncate the text if it is too long for the model to process.  This defaults
	// to "true" for Ollama.
	//
	// TODO: grab the Optional monad for this since the default value is not the zero value?
	// Truncate bool `json:"truncate"`

	// KeepAlive is how long the model should stay in memory after the request; this is a Go duration time string.
	KeepAlive time.Duration `json:"keep_alive,omitempty"`

	// Options is a map of parameters that override the model parameters, such as temperature.
	Options map[string]any `json:"options,omitempty"`
}

type Response struct {
	Model string `json:"model"`

	// Embeddings consists of one vector per input in the request.
	Embeddings [][]float32 `json:"embeddings"`

	TotalDuration   time.Duration `json:"total_duration"`
	LoadDuration    time.Duration `json:"load_duration"`
	PromptEvalCount int64         `json:"prompt_eval_count"`
}

// https://github.com/ollama/ollama/blob/main/docs/api.md#generate-a-chat-completion
//
