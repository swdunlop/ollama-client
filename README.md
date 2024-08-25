# Ollama Go Client Library

This package defines a partial Ollama client with rich feature set around chat requests with Ollama.
It was created to experiment with the new tool calling functionality in Llama.cpp and Ollama.

See [example/orderbot](example/orderbot/orderbot.go) for a full example of how to bind a Go function and provide it as a tool.
A simpler example of tool use can be found in [example/tick](example/tick/tick.go).

## Using the Library

First, you must install the client as a dependency of your project:

```sh
go install github.com/swdunlop/ollama-client
```

Then, in many circumstances, you can just use the `ollama.Chat` function.

```go
import "github.com/swdunlop/ollama-client"

ret, _ := ollama.Chat(
	context.TODO(),
	chat.Model(`llama3.1:latest`),
	chat.Message(`user`, `what is the airspeed of an unladen swallow?`),
)
```

This will connect to the [Ollama](https://ollama.com) instance running locally and run the request. See the [examples](./examples)
for more involved examples using tools and other features.
