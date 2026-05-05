// Collections example — demonstrates the workspace-scoped JSON
// document store via the Tavora SDK.
//
// Collections are mongo-style document buckets the agent uses for typed
// working memory (lists of leads, scraped rows, normalized records).
// Distinct from `stores` (RAG vector buckets) and from per-run `data`.
//
// This program walks every primitive on the SDK surface:
//
//  1. Insert a single doc
//  2. Bulk-insert several docs
//  3. Find with filter operators ($gte, $in, range)
//  4. Find with sort + limit
//  5. Update matching docs
//  6. Remove matching docs
//  7. List + drop
//
// Run it against a fresh workspace so the bucket names don't collide.
//
// Usage:
//
//	export TAVORA_URL=http://localhost:8080
//	export TAVORA_API_KEY=tvr_...
//	go run .
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	tavora "github.com/tavora-ai/tavora-sdk-go"
)

const collName = "leads"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	url := os.Getenv("TAVORA_URL")
	key := os.Getenv("TAVORA_API_KEY")
	if url == "" || key == "" {
		return fmt.Errorf("set TAVORA_URL and TAVORA_API_KEY environment variables")
	}

	client := tavora.NewClient(url, key)
	ctx := context.Background()

	// Start clean so reruns don't accumulate. Idempotent.
	if err := client.DropCollection(ctx, collName); err != nil {
		return fmt.Errorf("dropping pre-existing collection: %w", err)
	}

	// 1. Insert a single document.
	id, err := client.InsertCollectionDocument(ctx, collName, tavora.CollectionDocument{
		"company": "Acme Corp",
		"score":   90,
		"tier":    "enterprise",
		"contact": "alice@acme.example",
	})
	if err != nil {
		return fmt.Errorf("insert: %w", err)
	}
	fmt.Printf("Inserted Acme Corp → _id=%d\n", id)

	// 2. Bulk-insert.
	ids, err := client.InsertCollectionDocuments(ctx, collName, []tavora.CollectionDocument{
		{"company": "Globex", "score": 65, "tier": "smb"},
		{"company": "Initech", "score": 45, "tier": "smb"},
		{"company": "Umbrella", "score": 82, "tier": "midmarket"},
		{"company": "Hooli", "score": 30, "tier": "smb"},
	})
	if err != nil {
		return fmt.Errorf("insertMany: %w", err)
	}
	fmt.Printf("Bulk-inserted %d more docs (ids: %v)\n\n", len(ids), ids)

	// 3. Find with filter operators. Mongo-style operator object: $gte
	// (and friends $gt, $lt, $lte, $ne, $in) live inside a per-field map.
	hot, err := client.FindCollectionDocuments(ctx, collName, tavora.FindCollectionInput{
		Filter: map[string]any{
			"score": map[string]any{"$gte": 70},
		},
	})
	if err != nil {
		return fmt.Errorf("find $gte: %w", err)
	}
	fmt.Printf("Hot leads (score >= 70): %d\n", len(hot))
	printDocs(hot)

	// $in — match any value in the list.
	priority, err := client.FindCollectionDocuments(ctx, collName, tavora.FindCollectionInput{
		Filter: map[string]any{
			"tier": map[string]any{"$in": []any{"enterprise", "midmarket"}},
		},
	})
	if err != nil {
		return fmt.Errorf("find $in: %w", err)
	}
	fmt.Printf("\nPriority tier (enterprise OR midmarket): %d\n", len(priority))
	printDocs(priority)

	// 4. Sort + limit. Prefix the sort field with "-" for descending.
	top3, err := client.FindCollectionDocuments(ctx, collName, tavora.FindCollectionInput{
		Sort:  "-score",
		Limit: 3,
	})
	if err != nil {
		return fmt.Errorf("find sort+limit: %w", err)
	}
	fmt.Printf("\nTop 3 by score:\n")
	printDocs(top3)

	// 5. Update. Promote Globex to midmarket.
	updated, err := client.UpdateCollectionDocuments(ctx, collName, tavora.UpdateCollectionInput{
		Filter:  map[string]any{"company": "Globex"},
		Updates: map[string]any{"tier": "midmarket", "promoted": true},
	})
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	fmt.Printf("\nUpdated %d doc(s) (Globex → midmarket)\n", updated)

	// 6. Remove cold leads (score < 50).
	removed, err := client.RemoveCollectionDocuments(ctx, collName, tavora.RemoveCollectionInput{
		Filter: map[string]any{"score": map[string]any{"$lt": 50}},
	})
	if err != nil {
		return fmt.Errorf("remove: %w", err)
	}
	fmt.Printf("Removed %d cold lead(s)\n", removed)

	// 7. List collections + counts. Useful for dashboarding.
	colls, err := client.ListCollections(ctx)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}
	fmt.Printf("\nWorkspace collections:\n")
	for _, c := range colls {
		fmt.Printf("  %-20s %d docs\n", c.Name, c.Count)
	}

	return nil
}

// printDocs renders a small set of docs as JSON for the demo. Production
// code should pull typed fields directly off the map.
func printDocs(docs []tavora.CollectionDocument) {
	for _, d := range docs {
		b, _ := json.Marshal(d)
		fmt.Printf("  %s\n", string(b))
	}
}
