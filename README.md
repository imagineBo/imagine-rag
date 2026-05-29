# imagine-rag

[![Go Reference](https://pkg.go.dev/badge/github.com/imagineBo/imagine-rag.svg)](https://pkg.go.dev/github.com/imagineBo/imagine-rag)
[![CI](https://github.com/imagineBo/imagine-rag/actions/workflows/ci.yml/badge.svg)](https://github.com/imagineBo/imagine-rag/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/imagineBo/imagine-rag)](https://goreportcard.com/report/github.com/imagineBo/imagine-rag)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Official Go client SDK for the [imagine.bo](https://imagine.bo) RAG platform.

Build Retrieval-Augmented Generation (RAG) features — knowledge bases, document ingestion, site crawling, and AI-powered chat — without managing embeddings, vector stores, or LLM infrastructure yourself.

---

## Installation

```bash
go get github.com/imagineBo/imagine-rag
```

Requires Go 1.24 or later. No external dependencies — only the standard library.

---

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    imagine "github.com/imagineBo/imagine-rag"
)

func main() {
    ctx := context.Background()

    // 1. Create a client — WithBaseURL is required.
    //    Reuse this client across your whole application.
    client := imagine.New(
        "sk-your-api-key",
        imagine.WithBaseURL("https://your-server.com"),
    )

    // 2. Create a knowledge base and set it as the default.
    kb, err := client.CreateKB(ctx, "Product Docs", imagine.KBOpts{})
    if err != nil {
        log.Fatal(err)
    }
    client.SetActiveKB(kb.KBID)

    // 3. Ingest a PDF — processing is async on the server.
    result, err := client.IngestFile(ctx, "./manual.pdf", imagine.IngestOpts{})
    if err != nil {
        log.Fatal(err)
    }

    // 4. Wait until the file is ready (polls every 2 s by default).
    file, err := client.WaitForFile(ctx, result.FileID, 0)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("status:", file.Status) // "ready"

    // 5. Ask a question — tokens stream back as they are generated.
    stream, err := client.Query(ctx, "What is the return policy?", imagine.QueryOpts{})
    if err != nil {
        log.Fatal(err)
    }
    for chunk := range stream {
        if chunk.Err != nil {
            log.Fatal(chunk.Err)
        }
        fmt.Print(chunk.Text)
        if chunk.Done {
            break
        }
    }
}
```

---

## Client Configuration

`WithBaseURL` is **required** — `New()` panics without it. All other options are optional:

```go
client := imagine.New(
    "sk-your-api-key",
    imagine.WithBaseURL("https://your-server.com"), // required
    imagine.WithTimeout(120 * time.Second),          // optional
    imagine.WithKB("kb_abc123"),                     // optional
)
```

| Option | Required | Default | Description |
|---|---|---|---|
| `WithBaseURL(url)` | **yes** | — | Your hosted imagine.bo server URL |
| `WithTimeout(d)` | no | `30s` | HTTP request timeout (`0` = no timeout, useful for streams) |
| `WithHTTPClient(hc)` | no | built-in | Replace the underlying `http.Client` (proxies, test mocks) |
| `WithKB(kbID)` | no | `""` | Default knowledge base used when KBID is not set in opts |

---

## Function Reference

### Knowledge Bases

Manage isolated document collections.

```go
// Create a new KB.
kb, err := client.CreateKB(ctx, "Product Docs", imagine.KBOpts{
    Description: "All public documentation",
})
client.SetActiveKB(kb.KBID) // set as default

// List all KBs.
kbs, err := client.ListKBs(ctx)

// Delete a KB and everything inside it.
err = client.DeleteKB(ctx, kb.KBID)
```

---

### Ingest

Add content to a knowledge base. All calls return immediately — processing is async.

```go
// Upload a local file (PDF, DOCX, PPTX, TXT, MD, CSV).
result, err := client.IngestFile(ctx, "./docs/manual.pdf", imagine.IngestOpts{})

// Fetch and process a single web page.
result, err = client.IngestURL(ctx, "https://docs.acme.com/faq", imagine.IngestOpts{})

// Send a raw string directly.
result, err = client.IngestText(ctx, "Our policy is 30 days...", "return-policy", imagine.IngestOpts{})

// Submit multiple URLs at once — server processes them in parallel.
results, err := client.IngestURLBatch(ctx, []string{
    "https://docs.acme.com/page1",
    "https://docs.acme.com/page2",
}, imagine.IngestOpts{})
```

---

### Files

Inspect and remove ingested documents.

```go
// Poll until processing finishes (default interval: 2 s).
file, err := client.WaitForFile(ctx, result.FileID, 0)

// Check status without blocking.
file, err = client.GetFileStatus(ctx, fileID)

// List all files in a KB.
files, err := client.ListFiles(ctx, kbID)

// Delete a file and all its indexed chunks.
err = client.DeleteFile(ctx, fileID)
```

---

### Crawl

Spider an entire website into a knowledge base.

```go
// Start a crawl — returns immediately with a job ID.
job, err := client.CrawlURL(ctx, "https://docs.acme.com", imagine.CrawlOpts{
    MaxPages:   200,
    MaxDepth:   4,
    SameDomain: true,
})

// Wait until the crawl completes (polls every 5 s).
done, err := client.WaitForCrawl(ctx, job.JobID, 5*time.Second)
fmt.Printf("%d pages ingested\n", done.PagesDone)

// Check progress without blocking.
status, err := client.GetCrawlStatus(ctx, job.JobID)

// Stop a running crawl (already-ingested pages are kept).
err = client.CancelCrawl(ctx, job.JobID)
```

---

### Query

Ask questions against your knowledge base.

```go
// Streaming — tokens arrive as they are generated.
stream, err := client.Query(ctx, "How do I reset my password?", imagine.QueryOpts{
    TopK:    5,
    History: history, // []imagine.Message from your DB
})
for chunk := range stream {
    if chunk.Err != nil { log.Fatal(chunk.Err) }
    fmt.Print(chunk.Text)
    if chunk.Done { break }
}

// Synchronous — waits for the full answer.
resp, err := client.QuerySync(ctx, "What is the refund policy?", imagine.QueryOpts{})
fmt.Println(resp.Answer)
fmt.Println(resp.Sources) // document chunks used to answer

// Collect a stream into a QueryResponse.
stream, _ = client.Query(ctx, message, opts)
resp, err = imagine.CollectStream(stream)
```

---

### Sessions

The server stores the full message history per session, so your users can return to a previous chat.

```go
// 1. When a user starts a new chat — create a session and save the ID.
session, err := client.CreateSession(ctx, imagine.SessionOpts{Name: "Alice"})
// → save session.SessionID to your DB

// 2. When the user sends a message — load history, then query.
history, err := client.GetHistory(ctx, session.SessionID)
resp, err := client.QuerySync(ctx, userMessage, imagine.QueryOpts{
    History:   history,
    SessionID: session.SessionID, // server appends the new pair automatically
})

// 3. List all sessions (for a "past chats" UI).
sessions, err := client.ListSessions(ctx)

// 4. Re-open a previous chat.
session, err = client.GetSession(ctx, sessionID)
history, err = client.GetHistory(ctx, sessionID)

// 5. Clean up.
err = client.DeleteSession(ctx, sessionID)
```

---

## Error Handling

All non-2xx responses are returned as `*imagine.Error`:

```go
_, err := client.QuerySync(ctx, message, opts)
switch {
case err == nil:
    // success
case imagine.IsUnauthorized(err):
    // bad or expired API key
case imagine.IsQuotaExceeded(err):
    // monthly plan limit reached — upgrade on imagine.bo
case imagine.IsRateLimited(err):
    // too many requests — add back-off and retry
case imagine.IsNotFound(err):
    // KB / file / session does not exist
default:
    // inspect the raw error
    var apiErr *imagine.Error
    if errors.As(err, &apiErr) {
        fmt.Println(apiErr.StatusCode, apiErr.Code, apiErr.RequestID)
    }
}
```

---

## Running the Example

```bash
git clone https://github.com/imagineBo/imagine-rag
cd imagine-rag/example
API_KEY=sk-your-key SERVER_URL=https://your-server.com go run .
```

---

## Development

```bash
make test          # run all tests
make test-verbose  # run tests with -v
make cover         # generate and open HTML coverage report
make vet           # run go vet
make lint          # run golangci-lint (must be installed separately)
make build         # compile library + example
```

---

## License

[MIT](LICENSE)
