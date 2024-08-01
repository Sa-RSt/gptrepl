package main

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

type CompletionAPI interface {
	SendContext([]Message) (<-chan CompletionDelta, error)
}

type OpenAICompletionAPI struct {
	apiKey *string
	model  *string
}

func (capi *OpenAICompletionAPI) SendContext(ctx []Message) (<-chan CompletionDelta, error) {
	client := openai.NewClient(*capi.apiKey)
	background := context.Background()
	messages := make([]openai.ChatCompletionMessage, len(ctx))
	for i, msg := range ctx {
		messages[i] = openai.ChatCompletionMessage{Role: msg.Role, Content: msg.Content}
	}
	req := openai.ChatCompletionRequest{Model: *capi.model, Stream: true, Messages: messages}
	stream, err := client.CreateChatCompletionStream(background, req)
	if err != nil {
		return nil, fmt.Errorf("CreateChatCompletionStream: %v", err)
	}
	out := make(chan CompletionDelta, 32)
	go func() {
		defer close(out)
		defer stream.Close()
		for {
			response, err := stream.Recv()
			if err != nil {
				out <- CompletionDelta{delta: "", err: err}
				break
			}
			delta := response.Choices[0].Delta.Content
			out <- CompletionDelta{delta: delta, err: nil}
		}
	}()
	return out, nil
}
