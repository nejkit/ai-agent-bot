package provider

import (
	"context"
	"fmt"
	"github.com/nejkit/ai-agent-bot/models"
	"github.com/openai/openai-go"
)

type OpenAIClient struct {
	cli *openai.Client
}

func NewOpenAIClient(cli *openai.Client) *OpenAIClient {
	return &OpenAIClient{cli: cli}
}

func (o *OpenAIClient) SendMessagesToAI(ctx context.Context, messages []models.MessageData) (string, error) {
	var mappedMessages []openai.ChatCompletionMessageParamUnion

	for _, message := range messages {
		if message.CreatedBy == models.MessageTypeAssistant {
			mappedMessages = append(mappedMessages, openai.AssistantMessage(message.Text))
			continue
		}

		mappedMessages = append(mappedMessages, openai.UserMessage(message.Text))
	}

	comp, err := o.cli.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: openai.F(mappedMessages),
		Model:    openai.F(openai.ChatModelGPT4oMini),
	})

	if err != nil {
		fmt.Printf("error ai: %s", err.Error())
		return "", err
	}

	return comp.Choices[0].Message.Content, nil
	//return "test", nil
}
