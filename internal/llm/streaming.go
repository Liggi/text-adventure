package llm

import (
    "log"

    "github.com/openai/openai-go"
    "github.com/openai/openai-go/packages/ssestream"
)

type StreamChunk struct {
    Text  string
    Error error
    Done  bool
}

func ReadStreamChunks(stream *ssestream.Stream[openai.ChatCompletionChunk], debug bool) <-chan StreamChunk {
    chunks := make(chan StreamChunk)

    go func() {
        defer close(chunks)
        defer stream.Close()

        for stream.Next() {
            chunk := stream.Current()
            if len(chunk.Choices) > 0 {
                delta := chunk.Choices[0].Delta.Content
                if delta != "" {
                    if debug {
                        log.Printf("Stream chunk: %q", delta)
                    }
                    chunks <- StreamChunk{Text: delta}
                }
            }
        }

        if err := stream.Err(); err != nil {
            if debug {
                log.Printf("Stream error: %v", err)
            }
            chunks <- StreamChunk{Error: err, Done: true}
            return
        }

        if debug {
            log.Println("Stream finished")
        }
        chunks <- StreamChunk{Done: true}
    }()

    return chunks
}
