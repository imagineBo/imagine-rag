package imagine

import "time"

// -----------------------------------------------------------------------
// Shared
// -----------------------------------------------------------------------

// Message represents a single turn in a conversation.
// Tenant fetches these from their own DB and passes them to Query.
type Message struct {
	Role    string `json:"role"`    // "user" or "assistant"
	Content string `json:"content"`
}

// -----------------------------------------------------------------------
// Ingest
// -----------------------------------------------------------------------

// IngestOpts controls how a document is processed.
type IngestOpts struct {
	KBID         string            `json:"kb_id,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
	ChunkSize    int               `json:"chunk_size,omitempty"`
	ChunkOverlap int               `json:"chunk_overlap,omitempty"`
}

// IngestResult is returned immediately after submitting a document.
// Processing happens async on the server — poll GetFileStatus to check completion.
type IngestResult struct {
	FileID    string    `json:"file_id"`
	Status    string    `json:"status"` // "processing" | "ready" | "failed"
	CreatedAt time.Time `json:"created_at"`
}

// -----------------------------------------------------------------------
// Query
// -----------------------------------------------------------------------

// QueryOpts controls retrieval and generation behaviour.
type QueryOpts struct {
	KBID         string    `json:"kb_id,omitempty"`
	History      []Message `json:"history,omitempty"` // last N messages from tenant's DB
	SystemPrompt string    `json:"system_prompt,omitempty"`
	TopK         int       `json:"top_k,omitempty"`  // chunks to retrieve, default 5
	Temperature  float64   `json:"temperature,omitempty"`
	SessionID    string    `json:"session_id,omitempty"` // optional — server saves history when set
}

// Source is a document chunk that was used to answer the query.
type Source struct {
	FileID    string  `json:"file_id"`
	FileName  string  `json:"file_name"`
	ChunkText string  `json:"chunk_text"`
	Score     float64 `json:"score"`
}

// QueryResponse is returned by QuerySync.
type QueryResponse struct {
	Answer  string   `json:"answer"`
	Sources []Source `json:"sources"`
}

// StreamChunk is one piece of a streaming response.
type StreamChunk struct {
	Text    string   `json:"text"`    // incremental token(s)
	Done    bool     `json:"done"`    // true on the final chunk
	Sources []Source `json:"sources"` // only populated on the final chunk
	Err     error    `json:"-"`       // non-nil if stream broke
}

// -----------------------------------------------------------------------
// Crawl
// -----------------------------------------------------------------------

// CrawlOpts controls how a site is crawled.
type CrawlOpts struct {
	KBID       string            `json:"kb_id,omitempty"`
	MaxPages   int               `json:"max_pages,omitempty"`  // default: 100
	MaxDepth   int               `json:"max_depth,omitempty"`  // default: 3
	SameDomain bool              `json:"same_domain,omitempty"` // stay on seed domain
	Tags       map[string]string `json:"tags,omitempty"`
}

// CrawlJob represents the state of a crawl job.
type CrawlJob struct {
	JobID      string    `json:"job_id"`
	Status     string    `json:"status"`      // "queued" | "running" | "completed" | "failed" | "cancelled"
	SeedURL    string    `json:"seed_url"`
	PagesDone  int       `json:"pages_done"`
	PagesTotal int       `json:"pages_total"`
	KBID       string    `json:"kb_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// -----------------------------------------------------------------------
// Session
// -----------------------------------------------------------------------

// SessionOpts are optional fields when creating a session.
type SessionOpts struct {
	Name     string            `json:"name,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Session holds metadata for a single chat session stored on your server.
type Session struct {
	SessionID string            `json:"session_id"`
	Name      string            `json:"name"`
	Metadata  map[string]string `json:"metadata"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// -----------------------------------------------------------------------
// Files
// -----------------------------------------------------------------------

// File represents an ingested document stored on your server.
type File struct {
	FileID    string            `json:"file_id"`
	Name      string            `json:"name"`
	Status    string            `json:"status"` // "processing" | "ready" | "failed"
	KBID      string            `json:"kb_id"`
	Size      int64             `json:"size"`
	Tags      map[string]string `json:"tags"`
	CreatedAt time.Time         `json:"created_at"`
}

// -----------------------------------------------------------------------
// Knowledge Base
// -----------------------------------------------------------------------

// KBOpts are optional fields when creating a knowledge base.
type KBOpts struct {
	Description string            `json:"description,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// KnowledgeBase holds metadata for a knowledge base.
type KnowledgeBase struct {
	KBID        string            `json:"kb_id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Tags        map[string]string `json:"tags"`
	FileCount   int               `json:"file_count"`
	CreatedAt   time.Time         `json:"created_at"`
}
