// This program demonstrates every function in the imagine-rag SDK.
// Run it against a real server:
//
//	API_KEY=sk-your-key SERVER_URL=https://your-server.com go run .
//
// Or just read through it as a reference for your own integration.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	imagine "github.com/imagineBo/imagine-rag"
)

func main() {
	ctx := context.Background()

	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		log.Fatal("API_KEY environment variable is required")
	}

	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		log.Fatal("SERVER_URL environment variable is required (e.g. https://your-server.com)")
	}

	// -----------------------------------------------------------------------
	// 1. Create a client
	// -----------------------------------------------------------------------
	// New() is the only constructor. WithBaseURL is required.
	// Reuse the returned client across your app — it holds an HTTP connection
	// pool and is safe for concurrent use.
	client := imagine.New(apiKey,
		imagine.WithBaseURL(serverURL),

		// Increase timeout for large file uploads or long-running streams.
		imagine.WithTimeout(60*time.Second),
	)

	// -----------------------------------------------------------------------
	// 2. Knowledge Bases
	// -----------------------------------------------------------------------

	// CreateKB — create an isolated collection of documents.
	kb, err := client.CreateKB(ctx, "Product Docs", imagine.KBOpts{
		Description: "All public product documentation",
		Tags:        map[string]string{"env": "production"},
	})
	must(err)
	fmt.Printf("[KB] created: %s (%s)\n", kb.Name, kb.KBID)

	// SetActiveKB — every subsequent ingest/query call uses this KB by default.
	client.SetActiveKB(kb.KBID)

	// ListKBs — enumerate all knowledge bases for this API key.
	kbs, err := client.ListKBs(ctx)
	must(err)
	fmt.Printf("[KB] total: %d\n", len(kbs))

	// -----------------------------------------------------------------------
	// 3. Ingest — add content to the knowledge base
	// -----------------------------------------------------------------------
	// All four ingest functions return immediately. Processing is async —
	// use WaitForFile or GetFileStatus to check when a file is ready.

	// IngestFile — upload a local file (PDF, DOCX, PPTX, TXT, MD, CSV).
	fileResult, err := client.IngestFile(ctx, "./docs/manual.pdf", imagine.IngestOpts{
		// KBID is omitted — the active KB is used automatically.
	})
	must(err)
	fmt.Printf("[Ingest] file queued: %s\n", fileResult.FileID)

	// IngestURL — fetch and process a single web page.
	urlResult, err := client.IngestURL(ctx, "https://docs.acme.com/faq", imagine.IngestOpts{})
	must(err)
	fmt.Printf("[Ingest] URL queued: %s\n", urlResult.FileID)

	// IngestText — send a raw string directly (no file needed).
	textResult, err := client.IngestText(ctx,
		"Our return policy allows returns within 30 days, no questions asked.",
		"return-policy-v2",
		imagine.IngestOpts{},
	)
	must(err)
	fmt.Printf("[Ingest] text queued: %s\n", textResult.FileID)

	// IngestURLBatch — submit multiple URLs in one request; server processes
	// them in parallel. Returns one result per URL in the same order.
	batchResults, err := client.IngestURLBatch(ctx, []string{
		"https://docs.acme.com/getting-started",
		"https://docs.acme.com/api-reference",
	}, imagine.IngestOpts{})
	must(err)
	fmt.Printf("[Ingest] batch: %d URLs queued\n", len(batchResults))

	// -----------------------------------------------------------------------
	// 4. Files — track processing and manage ingested documents
	// -----------------------------------------------------------------------

	// WaitForFile — block until the file is "ready" or "failed".
	// Pass 0 for interval to use the default poll interval (2 s).
	file, err := client.WaitForFile(ctx, fileResult.FileID, 0)
	must(err)
	fmt.Printf("[File] %s is now: %s\n", file.Name, file.Status)

	// GetFileStatus — check a single file without blocking.
	urlFile, err := client.GetFileStatus(ctx, urlResult.FileID)
	must(err)
	fmt.Printf("[File] URL page status: %s\n", urlFile.Status)

	// ListFiles — list all files in a KB.
	files, err := client.ListFiles(ctx, kb.KBID)
	must(err)
	fmt.Printf("[File] total in KB: %d\n", len(files))

	// DeleteFile — remove a file and all its indexed chunks.
	err = client.DeleteFile(ctx, textResult.FileID)
	must(err)
	fmt.Println("[File] text file deleted")

	// -----------------------------------------------------------------------
	// 5. Crawl — spider an entire website into the KB
	// -----------------------------------------------------------------------

	// CrawlURL — start a crawl job. Returns immediately with a job ID.
	job, err := client.CrawlURL(ctx, "https://docs.acme.com", imagine.CrawlOpts{
		MaxPages:   200,
		MaxDepth:   4,
		SameDomain: true, // only follow links on docs.acme.com
	})
	must(err)
	fmt.Printf("[Crawl] started: %s\n", job.JobID)

	// GetCrawlStatus — check progress without blocking.
	status, err := client.GetCrawlStatus(ctx, job.JobID)
	must(err)
	fmt.Printf("[Crawl] status: %s (%d pages done)\n", status.Status, status.PagesDone)

	// WaitForCrawl — poll every 5 s until the crawl reaches a terminal state.
	// Use context.WithTimeout to set an overall deadline.
	crawlCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	done, err := client.WaitForCrawl(crawlCtx, job.JobID, 5*time.Second)
	must(err)
	fmt.Printf("[Crawl] finished: %s — %d pages ingested\n", done.Status, done.PagesDone)

	// CancelCrawl — stop a running crawl. Already-ingested pages are kept.
	job2, err := client.CrawlURL(ctx, "https://blog.acme.com", imagine.CrawlOpts{MaxPages: 50})
	must(err)
	err = client.CancelCrawl(ctx, job2.JobID)
	must(err)
	fmt.Printf("[Crawl] cancelled: %s\n", job2.JobID)

	// -----------------------------------------------------------------------
	// 6. Sessions — server-side chat history per user
	// -----------------------------------------------------------------------
	// The server stores messages so your users can return to a previous chat.
	// Save the SessionID in your own DB tied to your user.

	// CreateSession — call once when a user starts a new conversation.
	session, err := client.CreateSession(ctx, imagine.SessionOpts{
		Name:     "Alice — billing support",
		Metadata: map[string]string{"user_id": "usr_42"},
	})
	must(err)
	fmt.Printf("[Session] created: %s\n", session.SessionID)
	// → persist session.SessionID to your DB here

	// GetHistory — load all past messages before rendering or querying.
	history, err := client.GetHistory(ctx, session.SessionID)
	must(err)
	fmt.Printf("[Session] history length: %d\n", len(history))

	// QuerySync with SessionID — the server appends the new message pair
	// automatically, so GetHistory returns it on the next call.
	resp, err := client.QuerySync(ctx, "What is the refund policy?", imagine.QueryOpts{
		History:   history,
		SessionID: session.SessionID,
	})
	must(err)
	fmt.Println("[Query] sync answer:", resp.Answer)

	// ListSessions — show all sessions for this tenant.
	sessions, err := client.ListSessions(ctx)
	must(err)
	fmt.Printf("[Session] total: %d\n", len(sessions))

	// GetSession — re-open a previous chat by its ID.
	reOpened, err := client.GetSession(ctx, session.SessionID)
	must(err)
	fmt.Printf("[Session] re-opened: %s (last active: %s)\n",
		reOpened.Name, reOpened.UpdatedAt.Format(time.RFC3339))

	// DeleteSession — remove the session and all its messages permanently.
	err = client.DeleteSession(ctx, session.SessionID)
	must(err)
	fmt.Println("[Session] deleted")

	// -----------------------------------------------------------------------
	// 7. Query — streaming and synchronous
	// -----------------------------------------------------------------------

	// Query (streaming) — tokens arrive as they're generated.
	// Read from the channel until chunk.Done == true or chunk.Err != nil.
	stream, err := client.Query(ctx, "How do I reset my password?", imagine.QueryOpts{
		TopK: 5,
	})
	must(err)

	fmt.Print("[Query] streaming: ")
	for chunk := range stream {
		if chunk.Err != nil {
			log.Fatal(chunk.Err)
		}
		fmt.Print(chunk.Text)
		if chunk.Done {
			fmt.Printf("\n[Query] sources used: %d\n", len(chunk.Sources))
		}
	}

	// CollectStream — convenience wrapper that drains the channel into a
	// QueryResponse. Handy when you stream to a user but also need the full
	// assembled answer and sources list at the end.
	stream2, err := client.Query(ctx, "What are the shipping options?", imagine.QueryOpts{})
	must(err)
	collected, err := imagine.CollectStream(stream2)
	must(err)
	fmt.Println("[Query] collected:", collected.Answer)

	// -----------------------------------------------------------------------
	// 8. Error handling
	// -----------------------------------------------------------------------
	// All API errors are *imagine.Error with a typed Code string.
	// Use the helpers to avoid matching on raw strings.
	_, queryErr := client.QuerySync(ctx, "test", imagine.QueryOpts{})
	switch {
	case queryErr == nil:
		// success
	case imagine.IsUnauthorized(queryErr):
		log.Fatal("invalid or revoked API key")
	case imagine.IsQuotaExceeded(queryErr):
		log.Fatal("monthly quota reached — upgrade on imagine.bo")
	case imagine.IsRateLimited(queryErr):
		log.Fatal("rate limit hit — add back-off and retry logic")
	case imagine.IsNotFound(queryErr):
		log.Fatal("KB or file not found — check the ID")
	default:
		log.Fatal(queryErr)
	}

	// -----------------------------------------------------------------------
	// 9. Cleanup
	// -----------------------------------------------------------------------
	// DeleteKB removes the KB and every file / chunk it contains.
	err = client.DeleteKB(ctx, kb.KBID)
	must(err)
	fmt.Println("[KB] deleted")
}

// must logs a fatal error and exits if err is non-nil.
func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
