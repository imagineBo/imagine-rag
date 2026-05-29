package imagine

import (
	"context"
	"net/http"
)

// CreateSession creates a new chat session on the server and returns the session.
// Store the returned SessionID in your own DB tied to your user.
//
//	session, err := client.CreateSession(ctx, imagine.SessionOpts{Name: "Support chat"})
//	// save session.SessionID to your DB
func (c *Client) CreateSession(ctx context.Context, opts SessionOpts) (Session, error) {
	var session Session
	if err := c.do(ctx, http.MethodPost, "/v1/sessions", opts, &session); err != nil {
		return Session{}, err
	}
	return session, nil
}

// GetSession fetches metadata for an existing session.
//
//	session, err := client.GetSession(ctx, sessionID)
//	fmt.Println("session name:", session.Name)
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

// ListSessions returns all sessions belonging to this tenant's API key.
//
//	sessions, err := client.ListSessions(ctx)
//	for _, s := range sessions {
//	    fmt.Println(s.SessionID, s.Name)
//	}
func (c *Client) ListSessions(ctx context.Context) ([]Session, error) {
	var sessions []Session
	if err := c.do(ctx, http.MethodGet, "/v1/sessions", nil, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// GetHistory returns all messages stored for a session.
// Use this to load a previous conversation before rendering it or passing it to Query.
//
//	history, err := client.GetHistory(ctx, sessionID)
//	resp, err := client.QuerySync(ctx, userMessage, imagine.QueryOpts{History: history})
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

// DeleteSession removes the session and all its stored messages from the server.
//
//	err := client.DeleteSession(ctx, sessionID)
func (c *Client) DeleteSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return &Error{Code: ErrCodeInvalidRequest, Message: "sessionID must not be empty"}
	}
	return c.do(ctx, http.MethodDelete, "/v1/sessions/"+sessionID, nil, nil)
}
