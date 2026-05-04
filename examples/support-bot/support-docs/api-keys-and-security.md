# API Keys and Security

## API Key Overview

API keys authenticate SDK and REST API access to a specific space. Each key is scoped to one space and inherits that space's configuration.

Key format: `tvr_` followed by 64 hex characters (e.g., `tvr_a1b2c3d4...`)

## Creating API Keys

1. Go to Space Settings in the dashboard
2. Scroll to the "API Keys" section
3. Click "Create Key" and enter a descriptive name
4. Copy the key immediately — it is only shown once at creation time
5. The key is stored as a SHA-256 hash; we cannot retrieve the original key

We recommend creating separate keys for:
- Development and production environments
- Different services or applications
- Team members who need direct API access

## Using API Keys

### With the Go SDK
```go
client := sdk.NewClient("https://api.tavora.ai", "tvr_your_api_key")
```

### With the REST API
Include the key in the `X-API-Key` header:
```bash
curl -H "X-API-Key: tvr_your_api_key" https://api.tavora.ai/api/documents
```

## Rotating API Keys

To rotate a key without downtime:
1. Create a new API key
2. Update your application to use the new key
3. Verify the new key works correctly
4. Delete the old key

## Revoking API Keys

1. Go to Space Settings > API Keys
2. Click the delete button next to the key you want to revoke
3. The key is immediately invalidated — any requests using it will be rejected

## Security Best Practices

- **Never commit API keys to version control** — Use environment variables or a secrets manager
- **Use separate keys per environment** — Don't share keys between development and production
- **Rotate keys regularly** — We recommend rotating keys every 90 days
- **Monitor usage** — Check Token Usage in the dashboard for unexpected activity
- **Restrict access** — Only give API keys to team members who need them
- **Use HTTPS** — Always connect to the API over HTTPS in production

## Rate Limiting

API requests are rate-limited per space:
- Free tier: 60 requests per minute
- Pro plan: 600 requests per minute
- Enterprise: Custom limits

When you exceed the rate limit, the API returns HTTP 429 (Too Many Requests) with a `Retry-After` header indicating when you can retry.
