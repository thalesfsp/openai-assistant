package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/sashabaranov/go-openai"
)

////////////////////
// Configuration.
////////////////////

const (
	// RoleAssistant is the Bot's role in the conversation.
	RoleAssistant openai.ThreadMessageRole = "assistant"

	// RoleSystem is the Bot's instruction on how to proceed/interpret/behave
	// regarding the current conversation.
	RoleSystem openai.ThreadMessageRole = "system"

	// RoleUser is the User's role in the conversation.
	RoleUser openai.ThreadMessageRole = "user"
)

//nolint:go-revive
const (
	BotAssistantIDEnvVar = "BOT_ASSISTANT_ID"
	BotOpenAIKeyEnvVar   = "BOT_OPENAI_API_KEY"
	BotOpenAIOrgEnvVar   = "BOT_OPENAI_ORG_ID"
)

var (
	assistantID = loadFromEnvVar(true, BotAssistantIDEnvVar)
	openAIKey   = loadFromEnvVar(true, BotOpenAIKeyEnvVar, "OPENAI_API_KEY")
	openAIOrg   = loadFromEnvVar(false, BotOpenAIOrgEnvVar, "OPENAI_ORG_ID")
)

var (
	messageRegex = regexp.MustCompile(`msg_([a-zA-Z0-9]+)`)
	threadRegex  = regexp.MustCompile(`thread_([a-zA-Z0-9]+)`)
)

////////////////////
// Data structures.
////////////////////

// SubmitMessageResponse is the response from the SubmitMessage function.
type SubmitMessageResponse struct {
	CompletedRun      openai.Run          `json:"completedRun"`
	CreatedMessage    openai.Message      `json:"createdMessage"`
	ExecutionTime     time.Duration       `json:"executionTime"`
	ProcessedMessages []ProcessedMessage  `json:"processedMessages"`
	RawMessages       openai.MessagesList `json:"rawMessages"`
}

// ProcessedMessage is the processed message.
type ProcessedMessage struct {
	CreatedAt int    `json:"createdAt"`
	ID        string `json:"id"`
	Role      string `json:"role"`
	ThreadID  string `json:"threadID"`
	Value     string `json:"value"`
}

////////////////////
// Utilities.
////////////////////

// LoadFromEnvVar loads the value from the given environment variables.
func loadFromEnvVar(required bool, keys ...string) string {
	for _, key := range keys {
		value := os.Getenv(key)

		if value != "" {
			return value
		}
	}

	if required {
		panic(fmt.Sprintf("One of %v needs to be set", keys))
	}

	return ""
}

// retrieveOrCreateThread retrieves the thread if the thread ID is not empty,
// otherwise it creates a new thread.
func retrieveOrCreateThread(
	ctx context.Context,
	client *openai.Client,
	threadID string,
) (openai.Thread, error) {
	// Try to retrieve the thread, ONLY if the thread ID is not empty.
	if threadID != "" {
		thread, err := client.RetrieveThread(ctx, threadID)
		if err != nil {
			fmt.Println("Error retrieving thread:", err.Error())

			// Return a new thread if the thread does not exist.
			return client.CreateThread(ctx, openai.ThreadRequest{})
		}

		// Return the thread if it exists.
		return thread, nil
	}

	// Create a new thread.
	return client.CreateThread(ctx, openai.ThreadRequest{})
}

// CreateMessage creates a message in the given thread.
func createMessage(
	ctx context.Context,
	client *openai.Client,
	threadID string,
	role openai.ThreadMessageRole,
	content string,
) (openai.Message, error) {
	return client.CreateMessage(
		ctx,
		threadID,
		openai.MessageRequest{
			Role:    string(role),
			Content: content,
		},
	)
}

// Waits for the run to complete, or til the context is cancelled.
//
// NOTE: DO NOT USE context.Background() here, as it will never cancel!
func waitForRunCompletion(
	ctx context.Context,
	client *openai.Client,
	threadID, runID string,
) (openai.Run, error) {
	var (
		run openai.Run
		err error
	)

	for {
		// Retrieve the run.
		run, err = client.RetrieveRun(ctx, threadID, runID)
		if err != nil {
			return run, err
		}

		// Stop the loop if the run is completed.
		if run.Status == openai.RunStatusCompleted {
			break
		}

		// Wait for 1 second before checking again.
		//
		// NOTE: This should come from Configuration.
		time.Sleep(1 * time.Second)
	}

	return run, nil
}

// createRunAndRun creates a run and waits for it to complete.
func createRunAndRun(
	ctx context.Context,
	client *openai.Client,
	threadID string,
	assistantID string,
) (openai.Run, error) {
	run, err := client.CreateRun(
		ctx,
		threadID,
		openai.RunRequest{
			AssistantID: assistantID,
		},
	)
	if err != nil {
		return run, err
	}

	// NOTE: The duration should come from Configuration.
	queueCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if _, err := waitForRunCompletion(
		queueCtx,
		client,
		threadID,
		run.ID,
	); err != nil {
		return run, err
	}

	return run, nil
}

// ListMessages lists the messages in the given thread.
func listMessages(
	ctx context.Context,
	client *openai.Client,
	threadID string,
	limit *int,
	order *string,
	after *string,
	before *string,
) (openai.MessagesList, error) {
	msgs, err := client.ListMessage(
		ctx,
		threadID,
		limit,
		order,
		after,
		before,
	)
	if err != nil {
		return msgs, err
	}

	return msgs, nil
}

// SubmitMessage submits a message to the given thread.
func submitMessage(
	ctx context.Context,
	client *openai.Client,
	assistantID string,
	threadID string,
	role openai.ThreadMessageRole,
	content string,
	limit *int,
	order *string,
	after *string,
	before *string,
) (*SubmitMessageResponse, error) {
	start := time.Now()

	msg, err := createMessage(ctx, client, threadID, role, content)
	if err != nil {
		return nil, err
	}

	// NOTE: Don't use the same context for other operations.
	run, err := createRunAndRun(
		context.Background(), // Need to be here otherwise lint will fail.
		client,
		threadID,
		assistantID,
	)
	if err != nil {
		return nil, err
	}

	msgs, err := listMessages(
		ctx,
		client,
		threadID,
		limit,
		order,
		after,
		before,
	)
	if err != nil {
		return nil, err
	}

	return &SubmitMessageResponse{
		CompletedRun:      run,
		CreatedMessage:    msg,
		ExecutionTime:     time.Since(start),
		ProcessedMessages: processMessage(msgs),
		RawMessages:       msgs,
	}, nil
}

// ProcessMessage processes the messages.
func processMessage(msgs openai.MessagesList) []ProcessedMessage {
	processedMessages := []ProcessedMessage{}

	for _, message := range msgs.Messages {
		for _, content := range message.Content {
			// Ensure to add only messages with content.
			if content.Text == nil {
				continue
			}

			processedMessages = append(processedMessages, ProcessedMessage{
				CreatedAt: message.CreatedAt,
				ID:        message.ID,
				Role:      message.Role,
				ThreadID:  message.ThreadID,
				Value:     content.Text.Value,
			})
		}
	}

	return processedMessages
}

///////////////////
// Application starts here.
///////////////////

func main() {
	//////
	// CLI arguments, and validation.
	//////

	// Retrieves the value of the first argument.
	question := os.Args[1]

	if question == "" {
		panic("Question cannot be empty")
	}

	var threadID string
	var messageID string

	if len(os.Args) >= 3 {
		threadID = os.Args[2]

		// Validate the thread ID.
		if !threadRegex.MatchString(threadID) {
			panic("Invalid thread ID")
		}
	}

	if len(os.Args) >= 4 {
		messageID = os.Args[3]

		// Validate the message ID.
		if !messageRegex.MatchString(messageID) {
			panic("Invalid message ID")
		}
	}

	//////
	// OpenAI client setup.
	//////

	// NOTE: Use `NewOrgClient` in case you need to specify the organization.
	client := openai.NewClient(openAIKey)

	// This context is used for everything BUT the run.
	ctx, cancel := context.WithTimeout(
		context.Background(),
		60*time.Second,
	)
	defer cancel()

	//////
	// Assistant setup.
	//////

	assistant, err := client.RetrieveAssistant(
		ctx,
		assistantID,
	)
	if err != nil {
		panic("RetrieveAssistant: " + err.Error())
	}

	//////
	// Thread management.
	//////

	thread, err := retrieveOrCreateThread(
		ctx,
		client,
		threadID,
	)
	if err != nil {
		panic("retrieveOrCreateThread: " + err.Error())
	}

	//////
	// Message submission.
	//////

	// Default order.
	order := "asc"

	resp, err := submitMessage(
		ctx,
		client,
		assistant.ID,
		thread.ID,
		RoleUser,
		question,
		nil,
		&order,
		&messageID,
		nil,
	)
	if err != nil {
		panic("submitMessage: " + err.Error())
	}

	// Output as valid, and formatted JSON.
	processedMessagesJSON, err := json.MarshalIndent(resp.ProcessedMessages, "", "  ")
	if err != nil {
		panic("json.MarshalIndent: " + err.Error())
	}

	fmt.Println(string(processedMessagesJSON))
}
