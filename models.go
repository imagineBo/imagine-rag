package imagine

import "time"

// -----------------------------------------------------------------------
// Shared
// -----------------------------------------------------------------------

// Message represents a single turn in a conversation.
//
// Build a slice of Messages from your own database and pass it to [Query] or
// [QuerySync] via [QueryOpts.History] so the model has conversation context.
//
//	history := []imagine.Message{
//	    {Role: "user",      Content: "Hi, I need help."},
//	    {Role: "assistant", Content: "Sure! What can I do for you?"},
//	}
type Message struct {
	// Role is either "user" or "assistant".
	Role string `json:"role"`

	// Content is the text of the message.
	Content string `json:"content"`
}

// -----------------------------------------------------------------------
// Ingest
// -----------------------------------------------------------------------

// IngestOpts controls how a document is chunked and stored.
// All fields are optional — leave them zero to use server defaults.
type IngestOpts struct {
	// KBID is the knowledge base to ingest into.
	// Defaults to the client's active KB ([Client.SetActiveKB]) when empty.
	KBID string `json:"kb_id,omitempty"`

	// Tags are arbitrary key/value metadata attached to every chunk produced
	// from this document. Useful for tenant isolation or filtering.
	Tags map[string]string `json:"tags,omitempty"`

	// ChunkSize is the target number of tokens per chunk.
	// Leave 0 to use the server default (typically 512).
	ChunkSize int `json:"chunk_size,omitempty"`

	// ChunkOverlap is the number of tokens shared between adjacent chunks.
	// Leave 0 to use the server default (typically 64).
	ChunkOverlap int `json:"chunk_overlap,omitempty"`
}

// IngestResult is returned immediately when a document is submitted.
//
// Document processing (parsing, chunking, embedding) happens asynchronously
// on the server. Poll [Client.GetFileStatus] or call [Client.WaitForFile] to
// wait until the file is ready to query.
type IngestResult struct {
	// FileID uniquely identifies the ingested document.
	// Store this if you want to check status or delete the file later.
	FileID string `json:"file_id"`

	// Status is the initial processing state, usually "processing".
	// It transitions to "ready" or "failed" once the server finishes.
	Status string `json:"status"` // "processing" | "ready" | "failed"

	// CreatedAt is when the ingest job was accepted by the server.
	CreatedAt time.Time `json:"created_at"`
}

// -----------------------------------------------------------------------
// Query
// -----------------------------------------------------------------------

// QueryOpts controls retrieval and generation behaviour.
// All fields are optional — sensible defaults are applied server-side.
type QueryOpts struct {
	// KBID restricts retrieval to a single knowledge base.
	// Defaults to the client's active KB when empty.
	KBID string `json:"kb_id,omitempty"`

	// History is the conversation context passed to the model.
	// Fetch it from your DB ([Client.GetHistory]) or maintain it yourself.
	// The server appends both the new user message and its reply automatically
	// when SessionID is set.
	History []Message `json:"history,omitempty"`

	// SessionID tells the server to store the new message pair in this session.
	// When set, the server persists both the user message and the assistant
	// reply so future calls to [Client.GetHistory] return them.
	SessionID string `json:"session_id,omitempty"`

	// SystemPrompt overrides the default system instruction sent to the LLM.
	SystemPrompt string `json:"system_prompt,omitempty"`

	// TopK is the number of document chunks retrieved from the KB to use as
	// context. Higher values give more context but increase latency and cost.
	// Server default is 5.
	TopK int `json:"top_k,omitempty"`

	// Temperature controls the randomness of the generated answer (0.0–1.0).
	// Lower values are more deterministic; higher values are more creative.
	// Server default is 0.2.
	Temperature float64 `json:"temperature,omitempty"`
}

// Source is a document chunk that the server used to answer the query.
// Inspect these to show citations in your UI or audit retrieval quality.
type Source struct {
	// FileID is the ID of the parent document.
	FileID string `json:"file_id"`

	// FileName is the human-readable name of the parent document.
	FileName string `json:"file_name"`

	// ChunkText is the raw text of the retrieved chunk.
	ChunkText string `json:"chunk_text"`

	// Score is the semantic similarity between the query and this chunk
	// (higher is more relevant, typically between 0 and 1).
	Score float64 `json:"score"`
}

// QueryResponse is the result of a [Client.QuerySync] call.
type QueryResponse struct {
	// Answer is the full generated answer.
	Answer string `json:"answer"`

	// Sources lists the document chunks used to produce the answer.
	// Use these to render citations or "sources used" panels in your UI.
	Sources []Source `json:"sources"`
}

// StreamChunk is a single event in a [Client.Query] streaming response.
//
// Read chunks from the channel returned by [Client.Query] until Err is non-nil
// or Done is true:
//
//	for chunk := range stream {
//	    if chunk.Err != nil { /* handle error */ }
//	    fmt.Print(chunk.Text)
//	    if chunk.Done {
//	        // chunk.Sources is populated on the final chunk.
//	        break
//	    }
//	}
type StreamChunk struct {
	// Text contains incremental token(s) appended to the answer so far.
	Text string `json:"text"`

	// Done is true on the last chunk. Sources is populated at this point.
	Done bool `json:"done"`

	// Sources lists the document chunks used. Only set when Done == true.
	Sources []Source `json:"sources"`

	// Err is non-nil if the stream broke before completing.
	// Always check this field; a non-nil Err means the answer is incomplete.
	Err error `json:"-"`
}

// -----------------------------------------------------------------------
// Crawl
// -----------------------------------------------------------------------

// CrawlOpts controls how a website is crawled.
// All fields are optional — leave them zero to use server defaults.
type CrawlOpts struct {
	// KBID is the knowledge base to store crawled pages in.
	// Defaults to the client's active KB when empty.
	KBID string `json:"kb_id,omitempty"`

	// MaxPages is the upper limit on pages to crawl.
	// Server default is 100. Set higher for large documentation sites.
	MaxPages int `json:"max_pages,omitempty"`

	// MaxDepth is the maximum number of link hops from the seed URL.
	// Server default is 3.
	MaxDepth int `json:"max_depth,omitempty"`

	// SameDomain restricts the crawler to URLs on the same domain as the
	// seed URL. Strongly recommended to prevent runaway crawls.
	SameDomain bool `json:"same_domain,omitempty"`

	// Tags are arbitrary key/value metadata attached to every page ingested
	// by this crawl.
	Tags map[string]string `json:"tags,omitempty"`
}

// CrawlJob represents the state of a crawl job returned by [Client.CrawlURL],
// [Client.GetCrawlStatus], and [Client.WaitForCrawl].
type CrawlJob struct {
	// JobID uniquely identifies the crawl. Use it with [Client.GetCrawlStatus],
	// [Client.WaitForCrawl], and [Client.CancelCrawl].
	JobID string `json:"job_id"`

	// Status is the current state of the crawl.
	//
	//   - "queued"    — accepted, waiting for a crawler worker
	//   - "running"   — actively fetching pages
	//   - "done"      — all pages processed
	//   - "failed"    — crawler encountered an unrecoverable error
	//   - "cancelled" — stopped by [Client.CancelCrawl]
	Status string `json:"status"`

	// SeedURL is the starting URL provided to [Client.CrawlURL].
	SeedURL string `json:"seed_url"`

	// PagesDone is the number of pages successfully fetched and ingested.
	PagesDone int `json:"pages_done"`

	// PagesTotal is the estimated total number of pages to crawl.
	// This may be 0 until the crawler has discovered the full link graph.
	PagesTotal int `json:"pages_total"`

	// KBID is the knowledge base all crawled pages are stored in.
	KBID string `json:"kb_id"`

	// CreatedAt is when the crawl job was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is the last time the job state changed.
	UpdatedAt time.Time `json:"updated_at"`
}

// -----------------------------------------------------------------------
// Session
// -----------------------------------------------------------------------

// SessionOpts are optional fields when creating a session with [Client.CreateSession].
type SessionOpts struct {
	// Name is a human-readable label for the session (e.g. the user's name or
	// the first message preview). Purely cosmetic — not used during querying.
	Name string `json:"name,omitempty"`

	// Metadata is arbitrary key/value data you want to store alongside the
	// session (e.g. your internal user ID, locale, subscription tier).
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Session holds metadata for a single chat session stored on the server.
//
// Typical flow:
//  1. Call [Client.CreateSession] when a user starts a new chat.
//  2. Save Session.SessionID in your own database tied to your user.
//  3. On subsequent messages, call [Client.GetHistory] to load past messages,
//     then pass them to [Client.QuerySync] via [QueryOpts.History].
//  4. Call [Client.DeleteSession] when the user deletes the chat.
type Session struct {
	// SessionID is the unique identifier. Save this in your own DB.
	SessionID string `json:"session_id"`

	// Name is the human-readable label set at creation time.
	Name string `json:"name"`

	// Metadata is the arbitrary key/value data provided at creation time.
	Metadata map[string]string `json:"metadata"`

	// CreatedAt is when the session was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is the last time a message was added to the session.
	UpdatedAt time.Time `json:"updated_at"`
}

// -----------------------------------------------------------------------
// Files
// -----------------------------------------------------------------------

// File represents an ingested document stored in a knowledge base.
// Returned by [Client.ListFiles], [Client.GetFileStatus], and [Client.WaitForFile].
type File struct {
	// FileID uniquely identifies the file.
	FileID string `json:"file_id"`

	// Name is the original filename or the label passed to [Client.IngestText].
	Name string `json:"name"`

	// Status is the current processing state.
	//
	//   - "processing" — the file is being parsed, chunked, and embedded
	//   - "ready"      — chunks are indexed and the file can be queried
	//   - "failed"     — processing failed; check the dashboard for details
	Status string `json:"status"`

	// KBID is the knowledge base this file belongs to.
	KBID string `json:"kb_id"`

	// Size is the file size in bytes (0 for text ingested via [Client.IngestText]).
	Size int64 `json:"size"`

	// Tags are the metadata key/value pairs set in [IngestOpts.Tags].
	Tags map[string]string `json:"tags"`

	// CreatedAt is when the file was submitted for ingestion.
	CreatedAt time.Time `json:"created_at"`
}

// -----------------------------------------------------------------------
// Knowledge Base
// -----------------------------------------------------------------------

// KBOpts are optional fields when creating a knowledge base with [Client.CreateKB].
type KBOpts struct {
	// Description is a human-readable summary of what this KB contains.
	Description string `json:"description,omitempty"`

	// Tags are arbitrary key/value labels (e.g. {"env": "production"}).
	Tags map[string]string `json:"tags,omitempty"`
}

// KnowledgeBase holds metadata for a knowledge base.
// Returned by [Client.CreateKB], [Client.ListKBs], and related calls.
type KnowledgeBase struct {
	// KBID is the unique identifier. Pass it to [Client.SetActiveKB] or
	// include it in [IngestOpts.KBID] / [QueryOpts.KBID].
	KBID string `json:"kb_id"`

	// Name is the label provided to [Client.CreateKB].
	Name string `json:"name"`

	// Description is the optional summary provided to [Client.CreateKB].
	Description string `json:"description"`

	// Tags are the labels provided to [Client.CreateKB].
	Tags map[string]string `json:"tags"`

	// FileCount is the total number of documents currently in this KB.
	FileCount int `json:"file_count"`

	// CreatedAt is when the knowledge base was created.
	CreatedAt time.Time `json:"created_at"`
}
