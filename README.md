# imagine-rag

[![Go Reference](https://pkg.go.dev/badge/github.com/imagineBo/imagine-rag.svg)](https://pkg.go.dev/github.com/imagineBo/imagine-rag)
[![CI](https://github.com/imagineBo/imagine-rag/actions/workflows/ci.yml/badge.svg)](https://github.com/imagineBo/imagine-rag/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/imagineBo/imagine-rag)](https://goreportcard.com/report/github.com/imagineBo/imagine-rag)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Official Go client SDK for the [imagine.bo](https://imagine.bo) RAG platform.

The frontend **never calls the RAG server directly**. All communication goes through this library:

```
Frontend  →  Your Backend (uses this library)  →  RAG Server
```

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

## How the Library Calls the Server

Every request the library makes goes to the RAG server over HTTP. You never need to handle this yourself — it is all done internally.

### Request format

```
POST /v1/<endpoint>
Authorization: Bearer <api-key>
Content-Type: application/json
X-Imagine-SDK: go/1.0.0
```

### Response envelope

Every successful response from the server is wrapped in:

```json
{ "success": true, "data": <payload> }
```

The library unwraps this automatically. When you call `client.QuerySync(...)` you get back a `QueryResponse` directly — not the envelope.

### Error envelope

Every error response from the server looks like:

```json
{ "success": false, "error": { "code": "NOT_FOUND", "message": "..." } }
```

The library parses this into a typed `*imagine.Error` so you can handle it with the helper functions (`IsNotFound`, `IsUnauthorized`, etc.).

### Streaming (SSE)

For `client.Query(...)` the library sends `"stream": true` and reads the response as Server-Sent Events:

```
data: {"text":"Hello","done":false,"sources":[]}
data: {"text":" world","done":false,"sources":[]}
data: {"text":"","done":true,"sources":[{"file_id":"...","score":0.91,...}]}
data: [DONE]
```

Each event is decoded into a `StreamChunk` and sent to the channel you receive.

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

// Wait until the crawl finishes (polls every 5 s).
// Terminal statuses: "done", "failed", "cancelled"
done, err := client.WaitForCrawl(ctx, job.JobID, 5*time.Second)
fmt.Printf("%d pages ingested\n", done.PagesDone)

// Check progress without blocking.
status, err := client.GetCrawlStatus(ctx, job.JobID)

// Stop a running crawl (already-ingested pages are kept).
err = client.CancelCrawl(ctx, job.JobID)
```

`CrawlJob.Status` values:

| Value | Meaning |
|---|---|
| `queued` | Accepted, waiting for a crawler worker |
| `running` | Actively fetching pages |
| `done` | All pages processed successfully |
| `failed` | Crawler encountered an unrecoverable error |
| `cancelled` | Stopped by `CancelCrawl` |

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

The server stores the full message history per session, so users can return to a previous chat at any time. See [Session Workflows](#session-workflows) below for detailed frontend patterns.

```go
// Create a session when a user starts a new chat.
session, err := client.CreateSession(ctx, imagine.SessionOpts{Name: "Alice"})

// Load all messages for a session (chronological order).
history, err := client.GetHistory(ctx, session.SessionID)

// Query with session — server appends the new message pair automatically.
resp, err := client.QuerySync(ctx, userMessage, imagine.QueryOpts{
    History:   history,
    SessionID: session.SessionID,
})

// List all sessions for this tenant (for a "past chats" sidebar).
sessions, err := client.ListSessions(ctx)

// Re-fetch session metadata.
session, err = client.GetSession(ctx, sessionID)

// Delete a session and all its messages.
err = client.DeleteSession(ctx, sessionID)
```

---

## Session Workflows

This section covers every session-related action a frontend user can take.

### 1. User starts a new chat

Call once when the user opens a fresh conversation. Save the returned `SessionID` in your own database — you will need it for every subsequent operation on this chat.

```go
session, err := client.CreateSession(ctx, imagine.SessionOpts{
    Name:     "Support chat",                       // shown in the sidebar
    Metadata: map[string]string{"user_id": "u_42"}, // your own data
})
if err != nil {
    // handle error
}

// Persist to your DB:
//   INSERT INTO chats (session_id, user_id, name) VALUES (?, ?, ?)
//   session.SessionID, userID, session.Name
```

Then send the first message immediately:

```go
stream, err := client.Query(ctx, firstMessage, imagine.QueryOpts{
    SessionID: session.SessionID, // server stores user message + reply
})
// stream tokens to the frontend...
```

---

### 2. User opens the "Past Chats" sidebar

Fetch all sessions for the current tenant, ordered by most-recently-updated first. Render each one as a row in the sidebar.

```go
sessions, err := client.ListSessions(ctx)
if err != nil {
    // handle error
}

// Each imagine.Session contains:
//   session.SessionID  — use as the key when the user clicks a row
//   session.Name       — display label
//   session.UpdatedAt  — "last active" timestamp
//   session.CreatedAt  — creation timestamp
//   session.Metadata   — any extra data you stored at creation time

for _, s := range sessions {
    fmt.Printf("%s  |  %s  |  last active: %s\n",
        s.SessionID, s.Name, s.UpdatedAt.Format("Jan 2, 2006"))
}
```

---

### 3. User opens an older session

Two calls: one to get session metadata (name, timestamps) and one to load the full message history. Render the messages in chronological order, then let the user continue the conversation.

```go
// Step A — re-fetch metadata (name may have been updated, UpdatedAt reflects last activity).
session, err := client.GetSession(ctx, sessionID)
if err != nil {
    if imagine.IsNotFound(err) {
        // session was deleted — show an error in the UI
    }
    // handle other errors
}

// Step B — load the full message history.
history, err := client.GetHistory(ctx, sessionID)
if err != nil {
    // handle error
}

// history is []imagine.Message in chronological order:
//   []{Role:"user", Content:"..."}, {Role:"assistant", Content:"..."}}, ...
//
// Render each message in your chat UI.
for _, msg := range history {
    fmt.Printf("[%s] %s\n", msg.Role, msg.Content)
}
```

---

### 4. User sends a message in an existing session

Load history first (so the model has context), then call `Query` or `QuerySync` with both `History` and `SessionID`. The server automatically appends the new user message and the assistant reply to the session — no extra call needed.

```go
// Load the latest history before each message.
history, err := client.GetHistory(ctx, sessionID)
if err != nil {
    // handle error
}

// Stream the answer back to the frontend.
stream, err := client.Query(ctx, userMessage, imagine.QueryOpts{
    SessionID: sessionID, // persists the new pair to the session
    History:   history,   // gives the model conversation context
    TopK:      5,
})
if err != nil {
    // handle error
}

for chunk := range stream {
    if chunk.Err != nil {
        // stream broke — handle error
        break
    }
    // send chunk.Text to the frontend (WebSocket, SSE, etc.)
    fmt.Print(chunk.Text)
    if chunk.Done {
        // chunk.Sources contains the document chunks used to answer
        fmt.Printf("\nsources: %d\n", len(chunk.Sources))
        break
    }
}
```

> If you don't need streaming (e.g. a REST endpoint returning a full answer), use `QuerySync` instead:
>
> ```go
> resp, err := client.QuerySync(ctx, userMessage, imagine.QueryOpts{
>     SessionID: sessionID,
>     History:   history,
> })
> // resp.Answer — full answer string
> // resp.Sources — document chunks used
> ```

---

### 5. User deletes a session

Permanently removes the session and all its stored messages. Show a confirmation dialog in the UI before calling this.

```go
err := client.DeleteSession(ctx, sessionID)
if err != nil {
    if imagine.IsNotFound(err) {
        // already deleted — treat as success
        return nil
    }
    // handle other errors
}

// Remove from your local state / DB, redirect to a new chat.
```

---

### Complete session lifecycle (reference)

```
User action                     Library call
──────────────────────────────────────────────────────────────────
New chat button clicked      →  CreateSession(ctx, opts)
                                  └─ save SessionID to your DB

Sidebar opens                →  ListSessions(ctx)
                                  └─ render session list

User clicks a past chat      →  GetSession(ctx, sessionID)
                                  └─ show name / timestamps
                             →  GetHistory(ctx, sessionID)
                                  └─ render message bubbles

User sends a message         →  GetHistory(ctx, sessionID)   [refresh]
                             →  Query(ctx, msg, QueryOpts{
                                    SessionID: sessionID,
                                    History:   history,
                                })
                                  └─ stream tokens to UI

User deletes the chat        →  DeleteSession(ctx, sessionID)
                                  └─ remove from sidebar
```

---

## Error Handling

All non-2xx responses are returned as `*imagine.Error`. The server sends errors in this shape:

```json
{ "success": false, "error": { "code": "NOT_FOUND", "message": "..." } }
```

The library parses this automatically:

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
    var apiErr *imagine.Error
    if errors.As(err, &apiErr) {
        fmt.Println(apiErr.StatusCode, apiErr.Code, apiErr.Message)
        fmt.Println("request ID:", apiErr.RequestID) // include in support reports
    }
}
```

### Error code constants

| Constant | Server value | HTTP status |
|---|---|---|
| `imagine.ErrCodeUnauthorized` | `unauthorized` | 401 |
| `imagine.ErrCodeNotFound` | `NOT_FOUND` | 404 |
| `imagine.ErrCodeInvalidRequest` | `INVALID_REQUEST` | 400 |
| `imagine.ErrCodeServerError` | `INTERNAL_ERROR` | 500 |
| `imagine.ErrCodeRateLimited` | `rate_limited` | 429 |
| `imagine.ErrCodeQuotaExceeded` | `quota_exceeded` | — |
| `imagine.ErrCodeFileTooLarge` | `file_too_large` | — |
| `imagine.ErrCodeUnsupportedType` | `unsupported_type` | — |

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
