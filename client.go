package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"github.com/swdunlop/ollama-client/chat"
	"github.com/swdunlop/ollama-client/embed"
)

// With creates a new Ollama client or expands the previous one in a context.
func With(ctx context.Context, options ...Option) context.Context {
	if len(options) == 0 {
		return ctx
	}
	client := *from(ctx)
	client.requestHooks = append([]func(*http.Request) error(nil), client.requestHooks...)
	client.responseHooks = append([]func(*http.Response) error(nil), client.responseHooks...)
	for _, option := range options {
		option(&client)
	}
	return context.WithValue(ctx, ctxClient{}, &client)
}

// Chat does a chat request with the provided context.  If a toolkit is provided for the request, it will be used to
// handle any tool calls.
func Chat(ctx context.Context, options ...chat.Option) (*chat.Response, error) {
	req := newRequest[chat.Request](options...)
	toolkit := req.Toolkit()
	for {
		var rsp chat.Response
		err := from(ctx).Do(ctx, &rsp, `POST`, req, `/api/chat`)
		if err != nil {
			return nil, err
		}
		if toolkit == nil || len(rsp.Message.ToolCalls) == 0 {
			return &rsp, nil
		}
		for _, call := range rsp.Message.ToolCalls {
			msg, err := toolkit.Call(ctx, call)
			if err != nil {
				return &rsp, err
			}
			req.Messages = append(req.Messages, msg)
		}
	}
}

// Embed returns a vector that describes the input in a dimensions understood by the model.  This can be used to identify similar inputs
// or to find relevant inputs.
func Embed(ctx context.Context, options ...embed.Option) (*embed.Response, error) {
	req := newRequest[embed.Request](options...)
	var rsp embed.Response
	err := from(ctx).Do(ctx, &rsp, `POST`, req, `/api/embed`)
	if err != nil {
		return nil, err
	}
	return &rsp, nil
}

func newRequest[
	Req any,
	Option ~func(*Req),
](options ...Option) *Req {
	var req Req
	for _, option := range options {
		option(&req)
	}
	return &req
}

// Default is the default client for contexts without their own specialized client.
var Default = defaultClient

// New constructs a new Client with the provided options.
func New(options ...Option) *Client { return defaultClient.Apply(options...) }

// TraceZerolog adds a zerolog trace using the provided logger that traces requests and responses.
func TraceZerolog(logger zerolog.Logger) Option {
	return func(ct *Client) {
		ct.requestHooks = append(ct.requestHooks, func(req *http.Request) error {
			logger.Trace().Func(func(e *zerolog.Event) {
				e.Str(`method`, req.Method).Stringer(`url`, req.URL)
				body := stealBody(&req.Body)
				var msg json.RawMessage
				if err := json.Unmarshal(body, &msg); err == nil {
					e.RawJSON(`request`, msg)
				}
			}).Msg(`sending Ollama request`)
			return nil
		})
		ct.responseHooks = append(ct.responseHooks, func(rsp *http.Response) error {
			req := rsp.Request
			logger.Trace().Func(func(e *zerolog.Event) {
				e.Str(`method`, req.Method).Stringer(`url`, req.URL).Int(`status`, rsp.StatusCode)
				body := stealBody(&req.Body)
				var msg json.RawMessage
				if err := json.Unmarshal(body, &msg); err == nil {
					e.RawJSON(`response`, msg)
				}
			}).Msg(`received Ollama response`)
			return nil
		})
	}
}

func stealBody(rr *io.ReadCloser) []byte {
	var body []byte
	var err error
	switch r := (*rr).(type) {
	case nil:
		return nil
	case interface {
		io.ReadCloser
		Bytes() []byte
	}:
		body = r.Bytes()
		_ = r.Close()
	default:
		body, err = io.ReadAll(r)
	}
	if err != nil {
		panic(err) // TODO: shim in a reader that returns an error.
	}
	var bt bodyThief
	bt.Write(body)
	*rr = &bt
	return body
}

type bodyThief struct {
	bytes.Buffer
	err error
}

func (r *bodyThief) Close() error { return nil }
func (r *bodyThief) Read(p []byte) (int, error) {
	n, err := r.Buffer.Read(p)
	if err != nil && r.err != nil {
		err = r.err
	}
	return n, err
}

// RequestHook adds a request hook to the client; each request hook is applied to a request by the Do client method.
// This is useful for injecting authentication headers or logging.  They are applied in First In First Out (FIFO) order.
func RequestHook(hook func(*http.Request) error) Option {
	return func(ct *Client) { ct.requestHooks = append(ct.requestHooks, hook) }
}

// ResponseHook adds a response hook to the client; like request hooks, these hooks are applied to responses by the Do client
// method, but unlike RequestHooks, they are applied in Last In First Out (LIFO) order.
func ResponseHook(hook func(*http.Response) error) Option {
	return func(ct *Client) { ct.responseHooks = append(ct.responseHooks, hook) }
}

// Host specifies the base URL of the Ollama server.  This may be either a URL without a trailing "/" or a TCP/IP address,
// in which case, HTTP will be used.  The default host is `http://localhost:11434` but if OLLAMA_HOST is present in the
// environment, it will be used instead.
func Host(host string) Option {
	return func(ct *Client) { ct.ollamaHost = host }
}

type Option func(*Client)

type Client struct {
	// ollamaHost is the base URL of the Ollama server.  This should not have a trailing "/".  An address can be used, or
	// a URL.
	ollamaHost string

	requestHooks  []func(*http.Request) error
	responseHooks []func(*http.Response) error
}

var defaultClient = func() (ct Client) {
	if ct.ollamaHost = os.Getenv(`OLLAMA_HOST`); ct.ollamaHost == `` {
		ct.ollamaHost = "http://localhost:11434"
	}
	return
}()

// Apply returns a client improved with new options.
func (ct *Client) Apply(options ...Option) *Client {
	cp := *ct
	// ensure any slices are cloned so we do not share capacities
	for _, option := range options {
		option(&cp)
	}
	return &cp
}

// Do exchanges a Request for a Response or an error.
func (ct *Client) Do(ctx context.Context, rsp any, method string, req any, api string) error {
	url := ct.ollamaHost
	if strings.Contains(url, `://`) {
		url = strings.TrimSuffix(url, `/`)
	} else {
		url = `http://` + url
	}
	url += api

	var hreq *http.Request
	switch method {
	case `POST`, `PUT`, `PATCH`:
		requestJSON, err := json.Marshal(req)
		if err != nil {
			return err
		}
		// json.NewEncoder(os.Stdout).Encode(req)
		hreq, err = http.NewRequestWithContext(ctx, method, url, bytes.NewReader(requestJSON))
		if err != nil {
			return err
		}
		hreq.Header.Set(`Content-Length`, strconv.Itoa(len(requestJSON)))
		hreq.Header.Set(`Content-Type`, `application/json`)
	default:
		if req != nil {
			return fmt.Errorf(`unexpected %#T content for method %q`, req, method)
		}
		var err error
		hreq, err = http.NewRequestWithContext(ctx, method, url, nil)
		if err != nil {
			return err
		}
	}

	for _, hook := range ct.requestHooks {
		err := hook(hreq)
		if err != nil {
			return err
		}
	}

	hrsp, err := http.DefaultClient.Do(hreq)
	if err != nil {
		return err
	}
	for i := len(ct.responseHooks) - 1; i >= 0; i-- {
		err = ct.responseHooks[i](hrsp)
		if err != nil {
			return err
		}
	}
	defer hrsp.Body.Close()

	if hrsp.StatusCode < 200 || hrsp.StatusCode > 299 {
		content, _ := io.ReadAll(hrsp.Body)
		return &Error{
			URL:        url,
			StatusCode: hrsp.StatusCode,
			Status:     hrsp.Status,
			Header:     hrsp.Header,
			Content:    content,
		}
	}

	if rsp != nil {
		err = json.NewDecoder(hrsp.Body).Decode(rsp)
	}
	return err
}

type Error struct {
	URL        string
	StatusCode int
	Status     string
	Header     http.Header
	Content    []byte
}

func (err *Error) Error() string { return err.Status }

// hostURL tries to detect if the host is a URL or a network address and return an actual URL; it will return
// an empty string if the host does not match either.
func hostURL(host string) string {
	switch {
	case strings.Contains(host, `://`):
		// We assume it's a URL.
		return strings.TrimSuffix(host, `/`)
	default:
		return `http://` + host
	}
}

// Apply constructs an option that applies the provided options.
func Apply[T any](options ...func(*T)) func(*T) {
	return func(t *T) {
		for _, option := range options {
			option(t)
		}
	}
}

func from(ctx context.Context) *Client {
	client, _ := ctx.Value(ctxClient{}).(*Client)
	if client != nil {
		return client
	}
	return &Default
}

type ctxClient struct{}
