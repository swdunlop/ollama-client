// Package message provides options for altering a chat message for Ollama.
package message

import (
	"bytes"
	"image"
	"image/png"

	"github.com/swdunlop/ollama-client/chat/protocol"
)

// Image adds a Go image to a message by encoding it to PNG.
func Image(img image.Image) Option {
	var buf bytes.Buffer
	// Assuming one byte per pixel, which is generally a significant overallocation.
	bounds := img.Bounds()
	buf.Grow(bounds.Dx() * bounds.Dy())
	err := png.Encode(&buf, img)
	if err != nil {
		panic(err) // should never happen.
	}
	return PNG(buf.Bytes())
}

// PNG adds a PNG encoded image to a message, usable by multi-model models like `llava` and `bakllava`.`
func PNG(png []byte) Option {
	return func(m *protocol.Message) {
		m.Images = append(m.Images, protocol.Image(png))
	}
}

// An Option improves a message when applied to it.
type Option func(*protocol.Message)
