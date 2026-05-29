package imagine

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// Query sends a message to the RAG pipeline and returns a channel of streaming
// tokens. The channel is closed automatically when the stream ends.
//
// Each [StreamChunk] in the channel carries incremental text. On the final
// chunk (Chunk.Done == true), [StreamChunk.Sources] is populated with the
// document chunks used to produce the answer.
//
// Always check [StreamChunk.Err] on each iteration — a non-nil error means the
// connection broke and the answer is incomplete.
//
// Passing history from your own database gives the model conversation context:
//
//	// Load the last N messages from your DB.
//	history, _ := client.GetHistory(ctx, sessionID)
//
//	stream, err := client.Query(ctx, "How do I reset my password?", imagine.QueryOpts{
//	    History:   history,
//	    SessionID: sessionID, // server appends the new pair automatically
//	    TopK:      5,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	for chunk := range stream {
//	    if chunk.Err != nil {
//	        log.Fatal(chunk.Err)
//	    }
//	    fmt.Print(chunk.Text)
//	    if chunk.Done {
//	        fmt.Printf("\n\nSources used: %d\n", len(chunk.Sources))
//	        break
//	    }
//	}
func (c *Client) Query(ctx context.Context, message string, opts QueryOpts) (<-chan StreamChunk, error) {
	if message == "" {
		return nil, &Error{Code: ErrCodeInvalidRequest, Message: "message must not be empty"}
	}

	payload := c.buildQueryPayload(message, opts, true)

	body, err := c.doStream(ctx, "/v1/query", payload)
	if err != nil {
		return nil, err
	}

	ch := make(chan StreamChunk, 16)
	go func() {
		defer body.Close()
		readSSE(body, ch)
	}()

	return ch, nil
}

// QuerySync sends a message to the RAG pipeline and waits for the complete
// answer before returning.
//
// Use this instead of [Client.Query] when you don't need streaming — for
// example in background jobs, webhooks, or APIs where you return the full
// answer in a single HTTP response.
//
//	resp, err := client.QuerySync(ctx, "What is the refund policy?", imagine.QueryOpts{
//	    History: history,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(resp.Answer)
//	for _, src := range resp.Sources {
//	    fmt.Printf("  - %s (score: %.2f)\n", src.FileName, src.Score)
//	}
func (c *Client) QuerySync(ctx context.Context, message string, opts QueryOpts) (QueryResponse, error) {
	if message == "" {
		return QueryResponse{}, &Error{Code: ErrCodeInvalidRequest, Message: "message must not be empty"}
	}

	payload := c.buildQueryPayload(message, opts, false)

	var result QueryResponse
	if err := c.do(ctx, http.MethodPost, "/v1/query", payload, &result); err != nil {
		return QueryResponse{}, err
	}
	return result, nil
}

// CollectStream drains a [Client.Query] stream into a [QueryResponse].
//
// It is a convenience wrapper for callers that want to stream tokens to a user
// (e.g. via WebSocket) while still getting a final QueryResponse with the full
// assembled answer and Sources list.
//
//	stream, _ := client.Query(ctx, message, opts)
//	resp, err := imagine.CollectStream(stream)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("Full answer:", resp.Answer)
func CollectStream(ch <-chan StreamChunk) (QueryResponse, error) {
	var sb strings.Builder
	var sources []Source
	for chunk := range ch {
		if chunk.Err != nil {
			return QueryResponse{}, fmt.Errorf("imagine: stream error: %w", chunk.Err)
		}
		sb.WriteString(chunk.Text)
		if chunk.Done {
			sources = chunk.Sources
		}
	}
	return QueryResponse{Answer: sb.String(), Sources: sources}, nil
}

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// queryPayload is the request body sent to POST /v1/query.
type queryPayload struct {
	Message      string    `json:"message"`
	History      []Message `json:"history,omitempty"`
	SessionID    string    `json:"session_id,omitempty"`
	KBID         string    `json:"kb_id,omitempty"`
	SystemPrompt string    `json:"system_prompt,omitempty"`
	TopK         int       `json:"top_k,omitempty"`
	Temperature  float64   `json:"temperature,omitempty"`
	Stream       bool      `json:"stream"`
}

// buildQueryPayload assembles a queryPayload from the public-facing QueryOpts.
func (c *Client) buildQueryPayload(message string, opts QueryOpts, stream bool) queryPayload {
	kbID := opts.KBID
	if kbID == "" {
		kbID = c.activeKBID
	}
	return queryPayload{
		Message:      message,
		History:      opts.History,
		SessionID:    opts.SessionID,
		KBID:         kbID,
		SystemPrompt: opts.SystemPrompt,
		TopK:         opts.TopK,
		Temperature:  opts.Temperature,
		Stream:       stream,
	}
}
