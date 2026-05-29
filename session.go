package imagine

import (
	"context"
	"net/http"
)

// CreateSession creates a new chat session on the server and returns its
// metadata, including the SessionID you must save in your own database.
//
// Sessions let the server store the full message history for a conversation,
// so your users can leave and return to the same chat without you having to
// manage message storage yourself.
//
// Typical first-message flow:
//
//  1. Call CreateSession to get a SessionID.
//  2. Save the SessionID in your DB tied to your user/chat record.
//  3. Pass [QueryOpts.SessionID] on every Query call — the server appends
//     both the user message and the assistant reply automatically.
//
//	session, err := client.CreateSession(ctx, imagine.SessionOpts{
//	    Name: "Alice — billing support",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Persist session.SessionID → your DB here.
//	fmt.Println("session:", session.SessionID)
func (c *Client) CreateSession(ctx context.Context, opts SessionOpts) (Session, error) {
	var session Session
	if err := c.do(ctx, http.MethodPost, "/v1/sessions", opts, &session); err != nil {
		return Session{}, err
	}
	return session, nil
}

// GetSession fetches the metadata for an existing session.
//
// Use this to display session details (name, created date) in your UI without
// loading the full message history. To load messages, call [Client.GetHistory].
//
//	session, err := client.GetSession(ctx, sessionID)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("session name:", session.Name)
//	fmt.Println("last activity:", session.UpdatedAt)
func (c *Client) GetSession(ctx context.Context, sessionID string) (Session, error) {
	if sessionID == "" {
		return Session{}, &Error{Code: ErrCodeInvalidRequest, Message: "sessionID must not be empty"}
	}
	var session Session
	if err := c.do(ctx, http.MethodGet, "/v1/sessions/"+sessionID, nil, &session); err != nil {
		return Session{}, err
	}
	return session, nil
}

// ListSessions returns all sessions belonging to this tenant's API key,
// ordered by most recently updated first.
//
// Use this to build a "past chats" list in your UI.
//
//	sessions, err := client.ListSessions(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, s := range sessions {
//	    fmt.Printf("%s  %s  (last active: %s)\n", s.SessionID, s.Name, s.UpdatedAt.Format(time.RFC3339))
//	}
func (c *Client) ListSessions(ctx context.Context) ([]Session, error) {
	var sessions []Session
	if err := c.do(ctx, http.MethodGet, "/v1/sessions", nil, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// GetHistory returns all [Message] values stored for a session, ordered
// chronologically (oldest first).
//
// Call this when a user reopens an old chat to render prior messages, or pass
// the result directly to [QueryOpts.History] to give the model context.
//
//	history, err := client.GetHistory(ctx, sessionID)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Render messages in your UI, or feed them into the next Query call.
//	resp, err := client.QuerySync(ctx, userMessage, imagine.QueryOpts{
//	    History:   history,
//	    SessionID: sessionID,
//	})
func (c *Client) GetHistory(ctx context.Context, sessionID string) ([]Message, error) {
	if sessionID == "" {
		return nil, &Error{Code: ErrCodeInvalidRequest, Message: "sessionID must not be empty"}
	}
	var messages []Message
	if err := c.do(ctx, http.MethodGet, "/v1/sessions/"+sessionID+"/history", nil, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

// DeleteSession permanently removes the session and all its stored messages
// from the server.
//
// This is irreversible. After deletion, [Client.GetSession] and
// [Client.GetHistory] will return a not-found error for this ID.
//
//	err := client.DeleteSession(ctx, sessionID)
//	if err != nil && !imagine.IsNotFound(err) {
//	    log.Fatal(err)
//	}
func (c *Client) DeleteSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return &Error{Code: ErrCodeInvalidRequest, Message: "sessionID must not be empty"}
	}
	return c.do(ctx, http.MethodDelete, "/v1/sessions/"+sessionID, nil, nil)
}
