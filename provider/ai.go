package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

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
	mappedMessages := mapMessages(messages)

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
}

func (o *OpenAIClient) UploadFile(ctx context.Context, content []byte) (string, error) {
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

func (o *OpenAIClient) SendMessageWithFileToAI(ctx context.Context, messages []models.MessageData, fileId string) (string, string, error) {
	mappedMessages := mapThreadMessages(messages, fileId)

	response, err := o.cli.Beta.Threads.NewAndRun(
		ctx,
		openai.BetaThreadNewAndRunParams{
			AssistantID: openai.F(""),
			Model:       openai.F(openai.ChatModelGPT4oMini),
			Thread: openai.F(openai.BetaThreadNewAndRunParamsThread{
				Messages: openai.F(mappedMessages),
			}),
		},
	)

	if err != nil {
		fmt.Printf("error ai: %s", err.Error())
		return "", "", err
	}

	return response.ThreadID, response.ID, nil

}

func (o *OpenAIClient) PollResponseFromAssistant(ctx context.Context, threadId string, runId string) (string, string, error) {
	run, err := o.cli.Beta.Threads.Runs.PollStatus(ctx, threadId, runId, 2000)

	if err != nil {
		fmt.Printf("error ai: %s", err.Error())
		return "", "", err
	}

	if run.Status != "completed" {
		return "", "", errors.New(run.LastError.Message)
	}

	queryResult, err := o.cli.Beta.Threads.Messages.List(ctx, threadId, openai.BetaThreadMessageListParams{RunID: openai.F(runId)})

	if err != nil {
		fmt.Printf("error ai: %s", err.Error())
		return "", "", err
	}

	fileId := ""

	if queryResult.Data[0].Attachments != nil {
		fileId = queryResult.Data[0].Attachments[0].FileID
	}

	return queryResult.Data[0].Content[0].Text.Value, fileId, nil
}

func (o *OpenAIClient) DownloadFile(ctx context.Context, fileId string) ([]byte, error) {
	fileObject, err := o.cli.Files.Content(ctx, fileId)

	if err != nil {
		fmt.Printf("error ai: %s", err.Error())
		return nil, err
	}

	content := []byte{}

	_, err = fileObject.Body.Read(content)

	if err != nil {
		return nil, err
	}

	return content, nil
}

func mapThreadMessages(messages []models.MessageData, fileId string) []openai.BetaThreadNewAndRunParamsThreadMessage {
	ctxMessages := messages[:len(messages)-1]
	newMessage := messages[len(messages)-1]

	var result []openai.BetaThreadNewAndRunParamsThreadMessage

	for index := range ctxMessages {
		role := openai.BetaThreadNewAndRunParamsThreadMessagesRoleUser

		if ctxMessages[index].CreatedBy == models.MessageTypeAssistant {
			role = openai.BetaThreadNewAndRunParamsThreadMessagesRoleAssistant
		}

		result = append(result, openai.BetaThreadNewAndRunParamsThreadMessage{
			Role: openai.F(role),
			Content: openai.F(
				[]openai.MessageContentPartParamUnion{
					openai.MessageContentPartParam{
						Type: openai.F(openai.MessageContentPartParamTypeText),
						Text: openai.F(ctxMessages[index].Text),
					},
				},
			),
		})
	}

	result = append(
		result,
		openai.BetaThreadNewAndRunParamsThreadMessage{
			Role: openai.F(openai.BetaThreadNewAndRunParamsThreadMessagesRoleUser),
			Content: openai.F(
				[]openai.MessageContentPartParamUnion{
					openai.MessageContentPartParam{
						Type: openai.F(openai.MessageContentPartParamTypeText),
						Text: openai.F(newMessage.Text),
					},
				},
			),
			Attachments: openai.F([]openai.BetaThreadNewAndRunParamsThreadMessagesAttachment{
				{
					FileID: openai.F(fileId),
				},
			}),
		},
	)

	return result
}

func mapMessages(messages []models.MessageData) []openai.ChatCompletionMessageParamUnion {
	var mappedMessages []openai.ChatCompletionMessageParamUnion

	for _, message := range messages {
		if message.CreatedBy == models.MessageTypeAssistant {
			mappedMessages = append(mappedMessages, openai.AssistantMessage(message.Text))
			continue
		}

		mappedMessages = append(mappedMessages, openai.UserMessage(message.Text))
	}

	return mappedMessages
}
