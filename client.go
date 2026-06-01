package imagine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// defaultTimeout is applied to every HTTP request.
	// Use [WithTimeout] or [WithHTTPClient] to change it.
	defaultTimeout = 30 * time.Second

	// sdkVersion is sent in the X-Imagine-SDK header so the server can
	// track which SDK version made the request.
	sdkVersion = "go/1.0.0"
)

// Client is the entry point for all API calls.
//
// Create one with [New] and reuse it for the lifetime of your application —
// it holds an HTTP connection pool and is safe for concurrent use.
type Client struct {
	apiKey     string
	baseURL    string
	activeKBID string       // default KB used when opts.KBID is empty
	http       *http.Client
}

// Option is a functional option for [New].
// Pass one or more options to customise the client at construction time.
type Option func(*Client)

// WithBaseURL sets the base URL of your hosted imagine.bo server.
//
// This option is required. New panics if no base URL is provided.
// Every API call is sent to this server, so set it once and reuse the client.
//
//	client := imagine.New(apiKey, imagine.WithBaseURL("https://your-server.com"))
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(url, "/")
	}
}

// WithTimeout sets the HTTP request timeout applied to every call.
//
// The default is 30 s. For [Query] (streaming) you may want a longer timeout
// or 0 (no timeout) so the connection stays open for the full response stream.
//
//	// Never time out — useful for long-running streams.
//	client := imagine.New(apiKey, imagine.WithTimeout(0))
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.http.Timeout = d
	}
}

// WithHTTPClient replaces the underlying [net/http.Client] entirely.
//
// Use this when you need a custom transport (mutual TLS, corporate proxy,
// request signing) or want to inject a test double.
//
//	client := imagine.New(apiKey, imagine.WithHTTPClient(myCustomHTTPClient))
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.http = hc }
}

// WithKB sets a default knowledge base ID on the client.
//
// Every function that accepts [IngestOpts] or [QueryOpts] will use this KB
// when the caller leaves the KBID field empty. You can also change the default
// at runtime with [Client.SetActiveKB].
//
//	client := imagine.New(apiKey, imagine.WithKB("kb_abc123"))
func WithKB(kbID string) Option {
	return func(c *Client) { c.activeKBID = kbID }
}

// New creates and returns a new [Client].
//
// Both apiKey and [WithBaseURL] are required — New panics if either is missing.
// Obtain your API key from the imagine.bo dashboard.
// All other settings are optional and can be overridden with [Option] functions.
//
//	client := imagine.New(
//	    "sk-your-api-key",
//	    imagine.WithBaseURL("https://your-server.com"),
//	)
//
//	// With additional options:
//	client := imagine.New(
//	    "sk-your-api-key",
//	    imagine.WithBaseURL("https://your-server.com"),
//	    imagine.WithTimeout(60*time.Second),
//	    imagine.WithKB("kb_abc123"),
//	)
func New(apiKey string, opts ...Option) *Client {
	if apiKey == "" {
		panic("imagine: apiKey must not be empty")
	}
	c := &Client{
		apiKey: apiKey,
		http:   &http.Client{Timeout: defaultTimeout},
	}
	for _, o := range opts {
		o(c)
	}
	if c.baseURL == "" {
		panic("imagine: WithBaseURL is required — pass your server URL to New()")
	}
	return c
}

// SetActiveKB changes the default knowledge base ID at runtime.
//
// Equivalent to constructing the client with [WithKB]. Useful when you create
// a new KB after the client was already initialised.
//
//	kb, _ := client.CreateKB(ctx, "Docs", imagine.KBOpts{})
//	client.SetActiveKB(kb.KBID)
func (c *Client) SetActiveKB(kbID string) { c.activeKBID = kbID }

// GetActiveKB returns the current default knowledge base ID.
// Returns an empty string if none has been set.
func (c *Client) GetActiveKB() string { return c.activeKBID }

// -----------------------------------------------------------------------
// Internal HTTP helpers — not part of the public API.
// -----------------------------------------------------------------------

// do sends a JSON request and decodes the JSON response into out.
// out may be nil if the caller does not need the response body.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("imagine: marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("imagine: build request: %w", err)
	}
	c.setHeaders(req, "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("imagine: http: %w", err)
	}
	defer resp.Body.Close()

	return c.handleResponse(resp, out)
}

// doMultipart sends a multipart/form-data request (used for file uploads).
func (c *Client) doMultipart(ctx context.Context, path string, body io.Reader, contentType string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("imagine: build multipart request: %w", err)
	}
	c.setHeaders(req, contentType)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("imagine: http: %w", err)
	}
	defer resp.Body.Close()

	return c.handleResponse(resp, out)
}

// doStream sends a request and returns the raw response body for SSE reading.
// The caller is responsible for closing the returned body.
func (c *Client) doStream(ctx context.Context, path string, body any) (io.ReadCloser, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("imagine: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("imagine: build request: %w", err)
	}
	c.setHeaders(req, "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("imagine: http: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, c.parseError(resp)
	}

	return resp.Body, nil
}

// setHeaders attaches the Authorization, Content-Type, and SDK version headers.
func (c *Client) setHeaders(req *http.Request, contentType string) {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Imagine-SDK", sdkVersion)
}

// handleResponse checks the status code and decodes the JSON body into out.
// The server wraps every success payload in { "success": true, "data": <payload> }.
func (c *Client) handleResponse(resp *http.Response, out any) error {
	if resp.StatusCode >= 400 {
		return c.parseError(resp)
	}
	if out == nil {
		return nil
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("imagine: decode response: %w", err)
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("imagine: decode data: %w", err)
	}
	return nil
}

// parseError reads an error response body and returns a populated *Error.
// The server wraps errors in { "success": false, "error": { "code": "...", "message": "..." } }.
func (c *Client) parseError(resp *http.Response) error {
	apiErr := &Error{
		StatusCode: resp.StatusCode,
		RequestID:  resp.Header.Get("X-Request-ID"),
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err == nil {
		apiErr.Code = body.Error.Code
		apiErr.Message = body.Error.Message
	}
	// Fill in sensible defaults when the body was not structured JSON.
	if apiErr.Code == "" {
		switch resp.StatusCode {
		case 401:
			apiErr.Code = ErrCodeUnauthorized
		case 404:
			apiErr.Code = ErrCodeNotFound
		case 429:
			apiErr.Code = ErrCodeRateLimited
		default:
			apiErr.Code = ErrCodeServerError
		}
	}
	if apiErr.Message == "" {
		apiErr.Message = http.StatusText(resp.StatusCode)
	}
	return apiErr
}

// readSSE reads a Server-Sent Events stream from r and sends each event as a
// [StreamChunk] to ch. It closes ch when the stream ends or an error occurs.
// This function is intended to run in its own goroutine.
func readSSE(r io.Reader, ch chan<- StreamChunk) {
	defer close(ch)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			return
		}
		var chunk StreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			ch <- StreamChunk{Err: fmt.Errorf("imagine: parse sse chunk: %w", err)}
			return
		}
		ch <- chunk
		if chunk.Done {
			return
		}
	}
	if err := scanner.Err(); err != nil {
		ch <- StreamChunk{Err: fmt.Errorf("imagine: read stream: %w", err)}
	}
}
