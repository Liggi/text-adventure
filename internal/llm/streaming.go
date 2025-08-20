package llm

import (
	"errors"
	"io"
	"log"

	"github.com/sashabaranov/go-openai"
)

type StreamChunk struct {
	Text  string
	Error error
	Done  bool
}

func ReadStreamChunks(stream *openai.ChatCompletionStream, debug bool) <-chan StreamChunk {
	chunks := make(chan StreamChunk)
	
	go func() {
		defer close(chunks)
		defer stream.Close()
		
		for {
			response, err := stream.Recv()
			
			if errors.Is(err, io.EOF) {
				if debug {
					log.Println("Stream finished")
				}
				chunks <- StreamChunk{Done: true}
				return
			}
			
			if err != nil {
				if debug {
					log.Printf("Stream error: %v", err)
				}
				chunks <- StreamChunk{Error: err, Done: true}
				return
			}
			
			if len(response.Choices) > 0 && response.Choices[0].Delta.Content != "" {
				chunk := response.Choices[0].Delta.Content
				if debug {
					log.Printf("Stream chunk: %q", chunk)
				}
				chunks <- StreamChunk{Text: chunk}
			}
		}
	}()
	
	return chunks
}