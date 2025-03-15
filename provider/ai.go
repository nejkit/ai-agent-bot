package provider

import (
	"bytes"
	"context"
	"fmt"
	"github.com/nejkit/ai-agent-bot/models"
	"github.com/openai/openai-go"
	"io"
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

	if _, err = o.cli.Chat.Completions.Delete(ctx, comp.ID); err != nil {
		fmt.Printf("error ai: %s", err.Error())
		return "", err
	}

	return comp.Choices[0].Message.Content, nil
	//return "test", nil
}

func (o *OpenAIClient) DownloadFile(ctx context.Context, content []byte) (string, error) {
	reader := bytes.NewReader(content)

	fileObject, err := o.cli.Files.New(ctx, openai.FileNewParams{
		File:    openai.F[io.Reader](reader),
		Purpose: openai.F(openai.FilePurposeAssistants),
	})

	if err != nil {
		fmt.Printf("error ai: %s", err.Error())
		return "", err
	}

	return fileObject.ID, nil
}

func (o *OpenAIClient) SendMessageWithFileToAI(ctx context.Context) {

}
