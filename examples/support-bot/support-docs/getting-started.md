# Getting Started with Tavora

## Creating Your Account

1. Visit the Tavora dashboard and click "Get started"
2. Enter your email address and create a password (minimum 8 characters)
3. Verify your email by clicking the link sent to your inbox
4. You'll be prompted to create your first organization

## Setting Up Your First Space

A space is your isolated environment for documents, conversations, and agents. Each space has its own API key and configuration.

1. Log in to the dashboard and go to "Spaces" in the sidebar
2. Click "Create Space" and enter a name (e.g., "Production" or "Development")
3. Navigate to Space Settings to generate an API key
4. Copy the API key immediately — it will only be shown once

## Installing the Go SDK

```bash
go get github.com/tavora-ai/tavora-go/sdk
```

## Making Your First API Call

```go
client := sdk.NewClient("https://api.tavora.ai", "tvr_your_api_key")
space, err := client.GetSpace(ctx)
```

## Next Steps

- Upload documents to enable RAG-powered search
- Create conversations to chat with your knowledge base
- Set up agents for autonomous task execution
