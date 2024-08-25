package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/swdunlop/ollama-client"
	"github.com/swdunlop/ollama-client/chat"
	"github.com/swdunlop/ollama-client/chat/tool"
	"github.com/swdunlop/ollama-client/chat/toolkit"

	"github.com/markusmobius/go-dateparser"
)

func main() {
	err := run()
	if err != nil {
		println(`!!`, err.Error())
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	err := loadOrders()
	if err != nil {
		return err
	}

	findOrdersTool, err := tool.New(
		tool.Description(`Finds orders, applying various search parameters.`),
		tool.Func(findOrders),
		tool.Enum(`status`, `completed`, `delivering`, `preparing`, `pending`),
	)
	tk := toolkit.New(findOrdersTool)
	if err != nil {
		return err
	}

	options := []chat.Option{
		chat.Model(`mistral-nemo:12b-instruct-2407-q8_0`),
		chat.Temperature(0), // Do not imagine my food!
		chat.System(`Assist the user with inquiries about product orders using the provided tools.` +
			` Only use the tools provided, do not attempt to provide information that is not available.`,
		),
		chat.Tools(findOrdersTool),
		chat.User(strings.Join(os.Args[1:], " ")),
	}
	enc := json.NewEncoder(os.Stdout)
	for {
		ret, err := ollama.Chat(ctx,
			options...,
		)
		if err != nil {
			return err
		}
		enc.Encode(ret.Message)
		for _, call := range ret.Message.ToolCalls {
			ret, err := tk.Call(ctx, call)
			if err != nil {
				return err
			}
			enc.Encode(ret)
			options = append(options, chat.Message(ret.Role, ret.Content))
		}
		if ret.Message.Content != "" {
			println(ret.Message.Content)
		}
		if len(ret.Message.ToolCalls) == 0 {
			return nil
		}
	}
}

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

func loadOrders() error {
	data, err := os.ReadFile(filepath.Join(`example`, `orders.json`))
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var order Order
		err := dec.Decode(&order)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
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
