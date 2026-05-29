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
	defaultBaseURL = "https://api.imagine.bo"
	defaultTimeout = 30 * time.Second
	sdkVersion     = "go/1.0.0"
)

// Client is the main entry point. Create one with New() and reuse it.
type Client struct {
	apiKey     string
	baseURL    string
	activeKBID string
	http       *http.Client
}

// Option configures the Client.
type Option func(*Client)

// WithBaseURL overrides the API base URL.
// Default: https://api.imagine.bo
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(url, "/")
	}
}

// WithTimeout sets the HTTP request timeout.
// Default: 30s. For streaming responses use a longer timeout or 0 (no timeout).
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.http.Timeout = d
	}
}

// WithHTTPClient replaces the underlying HTTP client entirely.
// Useful for custom transports, proxies, or test mocks.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.http = hc }
}

// WithKB sets the default knowledge base ID used when none is specified in opts.
func WithKB(kbID string) Option {
	return func(c *Client) { c.activeKBID = kbID }
}

// New creates a Client. apiKey must not be empty.
//
//	client := imagine.New("sk-tenant-abc123")
//	client := imagine.New("sk-abc123", imagine.WithBaseURL("https://api.myhost.com"))
func New(apiKey string, opts ...Option) *Client {
	if apiKey == "" {
		panic("imagine: apiKey must not be empty")
	}
	c := &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: defaultTimeout},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// SetActiveKB changes the default KB at runtime.
func (c *Client) SetActiveKB(kbID string) { c.activeKBID = kbID }

// GetActiveKB returns the current default KB ID.
func (c *Client) GetActiveKB() string { return c.activeKBID }

// -----------------------------------------------------------------------
// internal HTTP helpers
// -----------------------------------------------------------------------

// do performs a JSON request and decodes the response into out.
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

// doMultipart performs a multipart/form-data request (used for file uploads).
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

// doStream performs a request and returns the raw response body for SSE reading.
// Caller is responsible for closing the body.
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

func (c *Client) setHeaders(req *http.Request, contentType string) {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Imagine-SDK", sdkVersion)
}

func (c *Client) handleResponse(resp *http.Response, out any) error {
	if resp.StatusCode >= 400 {
		return c.parseError(resp)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("imagine: decode response: %w", err)
	}
	return nil
}

func (c *Client) parseError(resp *http.Response) error {
	apiErr := &Error{
		StatusCode: resp.StatusCode,
		RequestID:  resp.Header.Get("X-Request-ID"),
	}
	// attempt to decode structured error body
	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err == nil {
		apiErr.Code = body.Code
		apiErr.Message = body.Message
	}
	// fill in defaults if body was not structured
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

// readSSE reads Server-Sent Events from r and sends StreamChunks to ch.
// Closes ch when done or on error.
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
