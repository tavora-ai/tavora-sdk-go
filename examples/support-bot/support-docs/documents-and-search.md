# Documents and Search

## Supported File Formats

Tavora supports the following document formats:
- **PDF** — Text is extracted automatically, including from scanned PDFs using OCR
- **Markdown** (.md) — Parsed with full formatting support
- **Plain text** (.txt) — Ingested as-is
- **CSV** — Each row is treated as a separate content chunk
- **HTML** — Text is extracted, tags are stripped

Maximum file size: 50 MB per document.

## Uploading Documents

### Via the Dashboard
1. Navigate to "Documents" in the sidebar
2. Select a store (or create a new one)
3. Click "Upload" and select your files
4. Documents are processed automatically — you'll see the status change from "pending" to "processing" to "ready"

### Via the SDK
```go
doc, err := client.UploadDocument(ctx, tavora.UploadDocumentInput{
    IndexID:  "store-id",
    FilePath: "/path/to/document.pdf",
})
```

## Document Processing Pipeline

When you upload a document, Tavora automatically:
1. **Extracts text** from the file format
2. **Chunks** the content into smaller pieces (default: 512 tokens with 50-token overlap)
3. **Generates embeddings** using Google's embedding model
4. **Indexes** the embeddings in pgvector for fast semantic search

Processing time depends on document size:
- Small documents (< 10 pages): ~5 seconds
- Medium documents (10-100 pages): ~30 seconds
- Large documents (100+ pages): 1-5 minutes

## Stores

Stores are collections of related documents. Use them to organize your knowledge base:
- Create separate indexes for different topics or departments
- When searching or chatting, you can scope queries to a specific store
- Deleting a store removes all its documents

## Semantic Search

Search finds documents by meaning, not just keyword matching:

```go
results, err := client.Search(ctx, tavora.SearchInput{
    Query:   "how do I reset my password?",
    IndexID: "store-id",  // optional: scope to a store
    TopK:    5,            // number of results
})
```

Each result includes:
- **Content** — The matching text chunk
- **Score** — Relevance score (0-1, higher is better)
- **Document ID** — The source document

## Troubleshooting

**Document stuck in "processing" status:**
- Large PDFs may take several minutes — wait and refresh
- If a document stays in "processing" for more than 10 minutes, try deleting and re-uploading it
- Check that the file is not corrupted or password-protected

**Search returns no results:**
- Ensure documents have finished processing (status: "ready")
- Try rephrasing your query — semantic search works best with natural language
- Check that you're searching in the correct store
