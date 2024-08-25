package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/swdunlop/ollama-client"
	"github.com/swdunlop/ollama-client/chat"
	"github.com/swdunlop/ollama-client/chat/tool"
	"github.com/swdunlop/ollama-client/chat/toolkit"

	"github.com/markusmobius/go-dateparser"
)

func main() {
	flag.BoolVar(&trace, `trace`, false, `trace HTTP requests`)
	flag.BoolVar(&trace, `json`, false, `trace HTTP requests`)
	flag.StringVar(&model, `model`, model, `name of ollama model to use, including tag`)
	flag.Parse()
	err := run()
	if err != nil {
		println(`!!`, err.Error())
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var options []ollama.Option
	if trace {
		var out io.Writer
		if outputJSON {
			out = os.Stderr
		} else {
			out = zerolog.ConsoleWriter{
				Out:        os.Stderr,
				TimeFormat: `2006-01-02 15:04:05`,
			}
		}
		logger := zerolog.New(out).Level(zerolog.TraceLevel).With().Timestamp().Logger()
		defer os.Stderr.Sync()

		options = append(options, ollama.TraceZerolog(logger))
	}
	ctx = ollama.With(ctx, options...)

	findOrdersTool, err := tool.New(
		tool.Description(`Finds orders, applying various search parameters.`),
		tool.Func(findOrders),
		tool.Enum(`status`, `completed`, `delivering`, `preparing`, `pending`),
	)
	tk := toolkit.New(findOrdersTool)
	if err != nil {
		return err
	}

	ret, err := ollama.Chat(ctx,
		chat.Model(model),
		chat.Temperature(0), // Do not imagine my food!
		chat.System(`Assist the user with inquiries about product orders using the provided tools.`+
			` Only use the tools provided, do not attempt to provide information that is not available.`,
		),
		chat.Toolkit(tk),
		chat.User(strings.Join(flag.Args(), " ")),
	)

	if err != nil {
		return err
	}
	if outputJSON {
		if !trace {
			os.Stdout.Sync()
			return json.NewEncoder(os.Stdout).Encode(ret)
		}
	}
	_, err = fmt.Println(ret.Message.Content)
	return err
}

var (
	trace      = false
	outputJSON = false
	model      = `llama3.1:8b-instruct-q6_K`
	// model      = `llama3.1:latest` is a lower q4 quant and seems to suffer a little from that.
	// model = `mistral-nemo:12b-instruct-2407-q8_0`
)

func findOrders(ctx context.Context, q struct {
	ID          tool.Optional[string] `json:"id"          type:"string"   use:"only return orders matching this ID"`
	Start       tool.Optional[Time]   `json:"start"       type:"datetime" use:"only return orders created on or after this time"`
	End         tool.Optional[Time]   `json:"end"         type:"datetime" use:"only return orders created before on or before this time"`
	Status      tool.Optional[string] `json:"status"      type:"string"   use:"only return orders with the specified status"`
	Description tool.Optional[string] `json:"description" type:"string"   use:"only return orders whose full text matches this description"`
}) ([]Order, error) {
	results := append([]Order(nil), orders...)
	if q.ID.Present() {
		id := q.ID.Value()
		results = where(results, func(order Order) bool { return order.ID == id })
	}
	if q.Start.Present() {
		time := q.Start.Value().Time()
		results = where(results, func(order Order) bool { return !order.Time.Before(time) })
	}
	if q.End.Present() {
		time := q.End.Value().Time()
		results = where(results, func(order Order) bool { return !order.Time.After(time) })
	}
	if q.Description.Present() {
		description := strings.ToLower(strings.TrimSpace(q.Description.Value()))
		results = where(results, func(order Order) bool {
			return strings.Contains(strings.ToLower(order.Description), description)
		})
	}
	if q.Status.Present() {
		status := q.Status.Value()
		results = where(results, func(order Order) bool { return order.Status == status })
	}
	return results, nil
}

func where[T any](seq []T, test func(T) bool) []T {
	ret := make([]T, 0, len(seq))
	for _, it := range seq {
		if test(it) {
			ret = append(ret, it)
		}
	}
	return ret
}

func init() {
	dec := json.NewDecoder(bytes.NewReader(ordersJSON))
	for {
		var order Order
		err := dec.Decode(&order)
		if err == io.EOF {
			return
		}
		if err != nil {
			panic(err)
		}
		orders = append(orders, order)
	}
}

var orders []Order

type Order struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Time        time.Time `json:"time"`
}

type Time string

func (t Time) Time() time.Time {
	date, err := dateparser.Parse(nil, string(t))
	if err != nil {
		return time.Time{}
	}
	return date.Time
}

//go:embed orders.json
var ordersJSON []byte
