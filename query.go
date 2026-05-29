package imagine

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// Query sends a message and returns a streaming channel of tokens.
// Tenant passes their conversation history fetched from their own DB.
// Read from the channel until chunk.Done == true or the channel closes.
// Check chunk.Err on each chunk — a non-nil Err means the stream broke.
//
//	// fetch last 10 messages from your own DB
//	history := db.GetLastMessages(sessionID, 10)
//
//	stream, err := client.Query(ctx, "How do I reset my password?", imagine.QueryOpts{
//	    History: history,
//	})
//	if err != nil { ... }
//
//	for chunk := range stream {
//	    if chunk.Err != nil { ... }
//	    fmt.Print(chunk.Text)
//	    if chunk.Done {
//	        // chunk.Sources has the document chunks used to answer
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

// QuerySync sends a message and waits for the complete answer.
// Simpler than Query — use this when you don't need streaming.
//
//	resp, err := client.QuerySync(ctx, "What is the refund policy?", imagine.QueryOpts{
//	    History: history,
//	})
//	fmt.Println(resp.Answer)
//	fmt.Println(resp.Sources)
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

// -----------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------

type queryPayload struct {
	Message      string    `json:"message"`
	History      []Message `json:"history,omitempty"`
	KBID         string    `json:"kb_id,omitempty"`
	SystemPrompt string    `json:"system_prompt,omitempty"`
	TopK         int       `json:"top_k,omitempty"`
	Temperature  float64   `json:"temperature,omitempty"`
	Stream       bool      `json:"stream"`
}

func (c *Client) buildQueryPayload(message string, opts QueryOpts, stream bool) queryPayload {
	kbID := opts.KBID
	if kbID == "" {
		kbID = c.activeKBID
	}
	return queryPayload{
		Message:      message,
		History:      opts.History,
		KBID:         kbID,
		SystemPrompt: opts.SystemPrompt,
		TopK:         opts.TopK,
		Temperature:  opts.Temperature,
		Stream:       stream,
	}
}

// CollectStream is a convenience helper that drains a Query stream into a
// QueryResponse. Useful when you want streaming output to the user but still
// need the final Sources list.
//
//	stream, _ := client.Query(ctx, message, opts)
//	resp, err := imagine.CollectStream(stream)
func CollectStream(ch <-chan StreamChunk) (QueryResponse, error) {
	var sb strings.Builder  // build answer text
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
