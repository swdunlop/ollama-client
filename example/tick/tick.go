package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/swdunlop/ollama-client"
	"github.com/swdunlop/ollama-client/chat"
	"github.com/swdunlop/ollama-client/chat/tool"
	"github.com/swdunlop/ollama-client/chat/toolkit"
)

/*
$ go run ./example/tick What time is it in Dublin?
It's 10:26 PM in Dublin.
*/
func main() {
	toolkit := toolkit.New(
		must(tool.New(
			tool.CamelNames(),
			tool.Func(now),
			tool.Description(`now returns the current time in the specified timezone, or UTC if the timezone is omitted`),
		)),
	)
	ret, err := ollama.Chat(
		// The ollama package use ollama.DefaultClient by default, but if there is a better client bound in to the Go
		// context, using ollama.With, it will use that instead.
		context.Background(),

		// Model specifies which model to use.  https://ollama.com/library has a list of models, but pay attention to
		// whether they support tool use.
		//
		// Note that you will need to `ollama pull llama3.1:latest` to use this model.
		chat.Model(`llama3.1:latest`),

		// Temperature influences how random the model can be in its response.  The lower the temperature, the more it will
		// adhere to the most predictable output.  In conversational applications, a higher temperature is desirable.  Each
		// model has its own sensitivity to temperature.
		//
		// Here, we set it to 0.
		chat.Temperature(0.0),

		// The Toolkit option supplies a set of tools that the model can use.  Chat will respond to calls for those tools
		// from the model automatically.
		chat.Toolkit(toolkit),

		// Message adds a message to the history in the chat request.  We are going to send two -- a system message that shapes
		// how the model will behave, and a user message that is the request from the user.
		chat.Message(`system`, `Use the provided tools to answer questions from the user about time.`+
			` Do not answer questions not related to time and make your answers as succinct as possible.`),
		chat.Message(`user`, strings.Join(os.Args[1:], " ")),
	)
	if err != nil {
		// There are a lot of reasons a chat request can fail, including:
		// - You do not have ollama running.
		// - You do not have the right model loaded.
		// - The model called the tool with an invalid timezone.  (This happens easily.)
		// - The model tried to call a tool that doesn't exist.  (This happens at really low quantizations or high temperatures.)
		fmt.Fprintln(os.Stderr, `!!`, err.Error())
	} else {
		fmt.Println(ret.Message.Content)
	}
}

// Now returns the current time in either UTC or the specified timezone.
func now(q struct {
	// TimeZone is a parameter.  Since we used "CamelNames" this is actually described as "timeZone" to the model.
	//
	// The tool.Optional here tells the toolkit that this is an optional field, and therefore the model should not be
	// told that it is required.
	TimeZone tool.Optional[string] `use:"time zone, such as America/New_York or Africa/Dakar" type:"string"`
}) (t time.Time, err error) {
	location := time.UTC
	if q.TimeZone.Present() {
		location, err = time.LoadLocation(q.TimeZone.Value())
		if err != nil {
			return
		}
	}
	return time.Now().In(location), nil
}

// must simply wraps the result of calling a common Go function that returns a value and an error, and panics.
//
// This is not a good pattern for production, but it is nice for demos.
func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
