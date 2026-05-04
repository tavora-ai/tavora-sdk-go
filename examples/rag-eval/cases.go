package main

// TestCase defines a RAG quality test case.
type TestCase struct {
	Name        string   // short identifier
	Query       string   // user question
	Expect      []string // keywords that must appear (case-insensitive)
	Description string   // what this validates
}

// retrievalCases test search quality — do the right chunks come back?
var retrievalCases = []TestCase{
	{
		Name:        "pricing-direct",
		Query:       "pricing plans",
		Expect:      []string{"Pro", "month"},
		Description: "Direct keyword match for pricing content",
	},
	{
		Name:        "pricing-semantic",
		Query:       "Tell me about the pricing model",
		Expect:      []string{"Pro", "Enterprise"},
		Description: "Semantic match: 'model' should find 'plan' content",
	},
	{
		Name:        "password-reset",
		Query:       "how to reset password",
		Expect:      []string{"password", "reset"},
		Description: "Procedural query for password reset",
	},
	{
		Name:        "file-formats",
		Query:       "what file formats are supported",
		Expect:      []string{"PDF", "Markdown", "CSV"},
		Description: "Factual retrieval of supported formats",
	},
	{
		Name:        "api-auth",
		Query:       "API authentication",
		Expect:      []string{"X-API-Key"},
		Description: "Technical detail retrieval",
	},
	{
		Name:        "rate-limits",
		Query:       "rate limiting",
		Expect:      []string{"requests per minute", "429"},
		Description: "Specific detail about rate limits",
	},
	{
		Name:        "key-rotation",
		Query:       "how to rotate keys",
		Expect:      []string{"new", "Delete"},
		Description: "Multi-step procedure for key rotation",
	},
	{
		Name:        "cancel-subscription",
		Query:       "cancel subscription",
		Expect:      []string{"Cancel", "30 days"},
		Description: "Semantic match: subscription ≈ plan",
	},
	{
		Name:        "mcp-servers",
		Query:       "MCP servers",
		Expect:      []string{"Model Context Protocol"},
		Description: "Technical concept retrieval",
	},
	{
		Name:        "troubleshoot-agent",
		Query:       "agent stuck or failing",
		Expect:      []string{"failed", "error"},
		Description: "Troubleshooting query",
	},
}

// e2eCases test the full RAG pipeline — does the LLM answer correctly?
var e2eCases = []TestCase{
	{
		Name:        "e2e-pricing-tiers",
		Query:       "What are the pricing tiers?",
		Expect:      []string{"Free", "Pro", "Enterprise"},
		Description: "LLM synthesizes tier info from chunks",
	},
	{
		Name:        "e2e-upload-pdf",
		Query:       "How do I upload a PDF?",
		Expect:      []string{"SDK", "upload"},
		Description: "Procedural answer from docs",
	},
	{
		Name:        "e2e-delete-account",
		Query:       "What happens when I delete my account?",
		Expect:      []string{"30 days", "deleted"},
		Description: "Specific fact extraction",
	},
	{
		Name:        "e2e-max-file-size",
		Query:       "What's the maximum file size for uploads?",
		Expect:      []string{"50 MB"},
		Description: "Precise number extraction",
	},
	{
		Name:        "e2e-2fa",
		Query:       "How do I enable two-factor authentication?",
		Expect:      []string{"authenticator", "QR code"},
		Description: "Multi-step procedure from docs",
	},
}
