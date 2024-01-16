package main

import (
	"context"
	"os"
	"testing"

	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
)

func Test_retrieveOrCreateThread(t *testing.T) {
	if os.Getenv("ENVIRONMENT") != "integration" {
		t.Skip("Skipping integration test")
	}

	// NOTE: If OrgID needs to be specified, replace `openai.NewClient` with
	// `openai.NewClientWithConfig`.
	// WARN: This will create a new thread every time the test is run!
	var openAIKey = loadFromEnvVar(true, "BOT_OPENAI_API_KEY", "OPENAI_API_KEY")

	ctx := context.Background()
	client := openai.NewClient(openAIKey)

	// Creates a brand new Thread.
	thread, err := retrieveOrCreateThread(ctx, client, "")
	assert.NoError(t, err)

	// NOTE: Despite being possible to delete A know thread by its ID,
	// Thread Management and retention for production usage is currently
	// "managed" by OpenAI. Current retention policy is 60 days.
	//
	// SEE: https://community.openai.com/t/assistants-api-do-assistant-thread-messages-expire/487558/10
	// SEE: https://platform.openai.com/docs/models/default-usage-policies-by-endpoint
	defer client.DeleteThread(ctx, thread.ID)

	// Retrieves the brand new Thread.
	retrievedThread, err := retrieveOrCreateThread(ctx, client, thread.ID)
	assert.NoError(t, err)

	// Ensure it's the same Thread.
	assert.Equal(t, thread.ID, retrievedThread.ID)

	// Creates another brand new Thread because the retrieve will fail.
	newThread, err := retrieveOrCreateThread(ctx, client, "123")
	assert.NoError(t, err)

	// NOTE: Despite being possible to delete A know thread by its ID,
	// Thread Management and retention for production usage is currently
	// "managed" by OpenAI. Current retention policy is 60 days.
	//
	// SEE: https://community.openai.com/t/assistants-api-do-assistant-thread-messages-expire/487558/10
	// SEE: https://platform.openai.com/docs/models/default-usage-policies-by-endpoint
	defer client.DeleteThread(ctx, newThread.ID)

	// Ensure it's not the same Thread.
	assert.NotEqual(t, thread.ID, newThread.ID)
}
