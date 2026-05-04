# Troubleshooting

## Common Issues

### "Unauthorized" (401) Error
- Your API key may be invalid or revoked — check Space Settings > API Keys
- Ensure you're using the correct key for the target space
- Verify the `X-API-Key` header (REST) or client initialization (SDK) is correct

### "Forbidden" (403) Error
- Your user account may not have the required role for this action
- API key operations require the "admin" role on the space
- Contact your organization admin to check your permissions

### "Rate Limited" (429) Error
- You've exceeded your plan's request limit
- Check the `Retry-After` response header for when to retry
- Consider upgrading your plan for higher limits
- Implement exponential backoff in your application

### "Space Not Found" (404) Error
- Verify the space ID in your API key is correct
- The space may have been deleted — check with your organization admin

## SDK Issues

### Connection Timeout
- Verify the Tavora server URL is correct and accessible
- Check your network/firewall settings
- The default timeout is 30 seconds — increase it for large document uploads

### Document Upload Fails
- Check the file size (maximum 50 MB)
- Verify the file format is supported (PDF, MD, TXT, CSV, HTML)
- Ensure the file is not corrupted or password-protected
- Check that you have sufficient storage on your plan

### Search Returns Empty Results
- Confirm documents have finished processing (status: "ready")
- Try broader or differently-worded queries
- Semantic search works best with natural language questions
- Check that you're querying the correct store

## Chat Issues

### Responses Are Not Using My Documents
- Enable RAG when sending messages: `UseRAG: true`
- Specify the store ID if you have multiple stores
- Verify documents are in "ready" status
- The RAG pipeline retrieves the top-k most relevant chunks — ensure your documents contain the information

### Responses Are Slow
- Large documents or many concurrent requests can increase response time
- Check Token Usage for your current consumption
- Consider using a faster model (e.g., Gemini Flash instead of Pro)

### Chat Context Is Lost
- Each conversation maintains its own history
- Starting a new conversation clears the context
- For long conversations, the system automatically manages context window limits

## Dashboard Issues

### Page Won't Load
- Clear your browser cache and reload
- Try a different browser
- Check the browser console for JavaScript errors
- Verify the backend server is running

### Data Not Updating
- Click the refresh button or reload the page
- Check your network connection
- If using the dev server, ensure the API proxy is configured correctly

## Getting Help

If you can't resolve your issue:

1. Check our documentation at /docs
2. Search existing issues on GitHub
3. Contact support at support@tavora.ai
4. Enterprise customers: Use your dedicated support channel

When contacting support, please include:
- Your organization and space name
- The error message or unexpected behavior
- Steps to reproduce the issue
- Your SDK version (`go list -m github.com/tavora-ai/tavora-go/sdk`)
