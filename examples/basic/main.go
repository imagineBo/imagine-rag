package main

import (
	"context"
	"fmt"
	"log"
	"time"

	imagine "github.com/imaginetools/imagine-rag"
)

func main() {
	ctx := context.Background()

	// -----------------------------------------------------------------------
	// 1. Create client
	// -----------------------------------------------------------------------
	client := imagine.New("sk-tenant-abc123",
		imagine.WithBaseURL("https://your-server.com"), // your hosted RAG server
	)

	// -----------------------------------------------------------------------
	// 2. Knowledge Base — create one and make it the default
	// -----------------------------------------------------------------------
	kb, err := client.CreateKB(ctx, "Product Docs", imagine.KBOpts{
		Description: "All public product documentation",
	})
	mustNot(err)
	fmt.Printf("Created KB: %s (%s)\n", kb.Name, kb.KBID)
	client.SetActiveKB(kb.KBID) // every ingest call now uses this KB automatically

	kbs, err := client.ListKBs(ctx)
	mustNot(err)
	fmt.Printf("Total KBs: %d\n", len(kbs))

	// -----------------------------------------------------------------------
	// 3. Ingest — upload content into the active KB
	// -----------------------------------------------------------------------

	// 3a. Upload a local file
	fileResult, err := client.IngestFile(ctx, "./docs/product-manual.pdf", imagine.IngestOpts{})
	mustNot(err)
	fmt.Printf("Queued file: %s (status: %s)\n", fileResult.FileID, fileResult.Status)

	// 3b. Ingest a single URL
	urlResult, err := client.IngestURL(ctx, "https://docs.acme.com/faq", imagine.IngestOpts{})
	mustNot(err)
	fmt.Printf("Queued URL: %s\n", urlResult.FileID)

	// 3c. Ingest raw text
	textResult, err := client.IngestText(ctx, "Our return policy is 30 days, no questions asked.", "return-policy", imagine.IngestOpts{})
	mustNot(err)
	fmt.Printf("Queued text: %s\n", textResult.FileID)

	// 3d. Ingest multiple URLs at once
	batchResults, err := client.IngestURLBatch(ctx, []string{
		"https://docs.acme.com/page1",
		"https://docs.acme.com/page2",
	}, imagine.IngestOpts{})
	mustNot(err)
	fmt.Printf("Queued %d URLs\n", len(batchResults))

	// -----------------------------------------------------------------------
	// 4. Files — wait for a file to finish processing, then inspect
	// -----------------------------------------------------------------------

	// Poll every 2 seconds until ready or failed
	file, err := client.WaitForFile(ctx, fileResult.FileID, 2*time.Second)
	mustNot(err)
	fmt.Printf("File %s is now %s\n", file.Name, file.Status)

	// List all files in the active KB
	files, err := client.ListFiles(ctx, kb.KBID)
	mustNot(err)
	fmt.Printf("Files in KB: %d\n", len(files))

	// Check a single file's status
	statusFile, err := client.GetFileStatus(ctx, urlResult.FileID)
	mustNot(err)
	fmt.Printf("URL file status: %s\n", statusFile.Status)

	// Delete a file
	err = client.DeleteFile(ctx, textResult.FileID)
	mustNot(err)
	fmt.Println("Deleted text file")

	// -----------------------------------------------------------------------
	// 5. Crawl — spider a whole site into the KB
	// -----------------------------------------------------------------------

	job, err := client.CrawlURL(ctx, "https://docs.acme.com", imagine.CrawlOpts{
		MaxPages:   200,
		MaxDepth:   4,
		SameDomain: true,
	})
	mustNot(err)
	fmt.Printf("Crawl started: %s\n", job.JobID)

	// Check progress once
	status, err := client.GetCrawlStatus(ctx, job.JobID)
	mustNot(err)
	fmt.Printf("Crawl status: %s (%d pages done)\n", status.Status, status.PagesDone)

	// Wait until the crawl finishes (polls every 5 s)
	doneCrawl, err := client.WaitForCrawl(ctx, job.JobID, 5*time.Second)
	mustNot(err)
	fmt.Printf("Crawl %s — %d pages ingested\n", doneCrawl.Status, doneCrawl.PagesDone)

	// Start a second crawl just to show CancelCrawl
	job2, err := client.CrawlURL(ctx, "https://blog.acme.com", imagine.CrawlOpts{MaxPages: 50})
	mustNot(err)
	err = client.CancelCrawl(ctx, job2.JobID)
	mustNot(err)
	fmt.Println("Cancelled crawl:", job2.JobID)

	// -----------------------------------------------------------------------
	// 6. Session — server stores chat history per session
	// -----------------------------------------------------------------------

	// Create a new session when a user opens the widget for the first time
	session, err := client.CreateSession(ctx, imagine.SessionOpts{
		Name: "Alice's support chat",
	})
	mustNot(err)
	fmt.Printf("Session created: %s\n", session.SessionID)
	// → save session.SessionID to YOUR database tied to your user

	// When the user sends a message, fetch history from the server first
	history, err := client.GetHistory(ctx, session.SessionID)
	mustNot(err)

	// Then query with that history — the server appends both messages automatically
	resp, err := client.QuerySync(ctx, "What is the refund policy?", imagine.QueryOpts{
		History:   history,
		SessionID: session.SessionID,
	})
	mustNot(err)
	fmt.Println("Answer:", resp.Answer)

	// List all sessions for this tenant
	sessions, err := client.ListSessions(ctx)
	mustNot(err)
	fmt.Printf("Total sessions: %d\n", len(sessions))

	// Re-open an old chat — fetch the full history to render in the UI
	session2, err := client.GetSession(ctx, session.SessionID)
	mustNot(err)
	fmt.Println("Re-opened session:", session2.Name)

	oldHistory, err := client.GetHistory(ctx, session.SessionID)
	mustNot(err)
	fmt.Printf("Loaded %d messages from history\n", len(oldHistory))

	// Clean up
	err = client.DeleteSession(ctx, session.SessionID)
	mustNot(err)
	fmt.Println("Session deleted")

	// -----------------------------------------------------------------------
	// 7. Query — streaming and sync
	// -----------------------------------------------------------------------

	// Streaming — tokens arrive as they're generated
	stream, err := client.Query(ctx, "How do I reset my password?", imagine.QueryOpts{
		TopK: 5,
	})
	mustNot(err)

	fmt.Print("Streaming answer: ")
	for chunk := range stream {
		if chunk.Err != nil {
			log.Fatal(chunk.Err)
		}
		fmt.Print(chunk.Text)
		if chunk.Done {
			fmt.Println()
			fmt.Printf("Used %d source chunks\n", len(chunk.Sources))
		}
	}

	// CollectStream — drains a streaming response into a QueryResponse
	stream2, err := client.Query(ctx, "What are the shipping options?", imagine.QueryOpts{})
	mustNot(err)
	collected, err := imagine.CollectStream(stream2)
	mustNot(err)
	fmt.Println("Collected answer:", collected.Answer)

	// -----------------------------------------------------------------------
	// 8. Error handling helpers
	// -----------------------------------------------------------------------
	_, err = client.QuerySync(ctx, "test", imagine.QueryOpts{})
	switch {
	case imagine.IsUnauthorized(err):
		log.Fatal("invalid API key")
	case imagine.IsQuotaExceeded(err):
		log.Fatal("plan quota hit — upgrade on imagine.bo")
	case imagine.IsNotFound(err):
		log.Fatal("resource not found")
	case imagine.IsRateLimited(err):
		log.Fatal("rate limited — slow down requests")
	}

	// -----------------------------------------------------------------------
	// 9. Cleanup — delete the KB (removes all its files and chunks)
	// -----------------------------------------------------------------------
	err = client.DeleteKB(ctx, kb.KBID)
	mustNot(err)
	fmt.Println("KB deleted")
}

func mustNot(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
