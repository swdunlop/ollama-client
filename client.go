package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/swdunlop/ollama-client/chat"
)

// With creates a new Ollama client or expands the previous one in a context.
func With(ctx context.Context, options ...Option) context.Context {
	panic(`TODO`)
}

// Chat does a chat request with the provided context.  Note that this does not handle a streaming result
// from chat.
func Chat(ctx context.Context, options ...chat.Option) (*chat.Response, error) {
	return do[*chat.Request, *chat.Response](ctx, options...)
}

func do[
	ReqP interface {
		*Req
		Request
	},
	RspP interface {
		*Rsp
	},
	Req any,
	Rsp any,
	Option ~func(ReqP),
](
	ctx context.Context, options ...Option,
) (RspP, error) {
	var req Req
	for _, option := range options {
		option(&req)
	}
	ret, err := Do(ctx, (ReqP)(&req))
	if err != nil {
		return nil, err
	}
	return ret.(*Rsp), nil
}

// Do does a request using the Ollama client from the current context or the default one.
func Do(ctx context.Context, req Request) (any, error) { return from(ctx).Do(ctx, req) }

// Default is the default client for contexts without their own specialized client.
var Default = defaultClient

// New constructs a new Client with the provided options.
func New(options ...Option) *Client { return defaultClient.Apply(options...) }

type Option func(*Client)

type Client struct {
	// OllamaHost is the base URL of the Ollama server.  This should not have a trailing "/".  An address can be used, or
	// a URL.
	OllamaHost string `json:"ollama_host" env:"OLLAMA_HOST" default:"http://localhost:11434"`
}

var defaultClient = func() (ct Client) {
	if ct.OllamaHost = os.Getenv(`OLLAMA_HOST`); ct.OllamaHost == `` {
		ct.OllamaHost = "http://localhost:11434"
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
func (ct *Client) Do(ctx context.Context, req Request) (any, error) {
	url := ct.OllamaHost
	if strings.Contains(url, `://`) {
		url = strings.TrimSuffix(url, `/`)
	} else {
		url = `http://` + url
	}
	method, api := req.OllamaAPI()
	url += api

	var hreq *http.Request
	switch method {
	case `POST`, `PUT`, `PATCH`:
		requestJSON, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}
		// json.NewEncoder(os.Stdout).Encode(req)
		hreq, err = http.NewRequestWithContext(ctx, method, url, bytes.NewReader(requestJSON))
		if err != nil {
			return nil, err
		}
		hreq.Header.Set(`Content-Length`, strconv.Itoa(len(requestJSON)))
		hreq.Header.Set(`Content-Type`, `application/json`)
	default:
		var err error
		hreq, err = http.NewRequestWithContext(ctx, method, url, nil)
		if err != nil {
			return nil, err
		}
	}

	hrsp, err := http.DefaultClient.Do(hreq)
	if err != nil {
		return nil, err
	}
	defer hrsp.Body.Close()

	rep := req.OllamaResponse(hrsp.StatusCode)
	if rep == nil {
		return nil, fmt.Errorf(`%w in response to %q`, hrsp.Status, api)
	}
	err = json.NewDecoder(hrsp.Body).Decode(rep)
	return rep, err
}

// A Request describes an Ollama request that can be delivered using the low level Do method of the client.
type Request interface {
	// OllamaAPI returns the method and API path + query to the Ollama API for this request.
	// If the method is a "POST", "PUT" or "PATCH", the request will be sent as application/json content.
	OllamaAPI() (method, api string)

	// OllamaResponse returns an empty Ollama response structure associated with the specified status code.
	OllamaResponse(status int) any
}

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
