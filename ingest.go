package imagine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

// IngestFile uploads a local file to be chunked, embedded, and stored.
// Supported formats: PDF, DOCX, TXT, MD, CSV, PPTX.
// Returns immediately — processing is async. Use WaitForFile or GetFileStatus to poll.
//
//	result, err := client.IngestFile(ctx, "./docs/manual.pdf", imagine.IngestOpts{})
func (c *Client) IngestFile(ctx context.Context, filePath string, opts IngestOpts) (IngestResult, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return IngestResult{}, fmt.Errorf("imagine: open file: %w", err)
	}
	defer f.Close()

	// encode opts as JSON to send alongside the file
	optsJSON, err := json.Marshal(c.applyKBDefault(opts))
	if err != nil {
		return IngestResult{}, fmt.Errorf("imagine: marshal opts: %w", err)
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// file field
	mimeType := mimeFromExt(filepath.Ext(filePath))
	part, err := createFormFile(mw, "file", filepath.Base(filePath), mimeType)
	if err != nil {
		return IngestResult{}, fmt.Errorf("imagine: create form file: %w", err)
	}
	if _, err = io.Copy(part, f); err != nil {
		return IngestResult{}, fmt.Errorf("imagine: write file to form: %w", err)
	}

	// options field
	if err = mw.WriteField("options", string(optsJSON)); err != nil {
		return IngestResult{}, fmt.Errorf("imagine: write options field: %w", err)
	}
	mw.Close()

	var result IngestResult
	if err = c.doMultipart(ctx, "/v1/ingest/file", &buf, mw.FormDataContentType(), &result); err != nil {
		return IngestResult{}, err
	}
	return result, nil
}

// IngestURL tells the server to fetch and process a publicly accessible URL.
// Returns immediately — processing is async.
//
//	result, err := client.IngestURL(ctx, "https://docs.acme.com/faq", imagine.IngestOpts{})
func (c *Client) IngestURL(ctx context.Context, url string, opts IngestOpts) (IngestResult, error) {
	if url == "" {
		return IngestResult{}, &Error{Code: ErrCodeInvalidRequest, Message: "url must not be empty"}
	}
	payload := struct {
		URL     string      `json:"url"`
		Options IngestOpts  `json:"options"`
	}{
		URL:     url,
		Options: c.applyKBDefault(opts),
	}
	var result IngestResult
	if err := c.do(ctx, http.MethodPost, "/v1/ingest/url", payload, &result); err != nil {
		return IngestResult{}, err
	}
	return result, nil
}

// IngestText sends raw string content directly.
// name is a human-readable label shown in the admin panel (e.g. "Q3 release notes").
//
//	result, err := client.IngestText(ctx, markdownContent, "release-notes-q3", imagine.IngestOpts{})
func (c *Client) IngestText(ctx context.Context, content, name string, opts IngestOpts) (IngestResult, error) {
	if content == "" {
		return IngestResult{}, &Error{Code: ErrCodeInvalidRequest, Message: "content must not be empty"}
	}
	payload := struct {
		Content string     `json:"content"`
		Name    string     `json:"name"`
		Options IngestOpts `json:"options"`
	}{
		Content: content,
		Name:    name,
		Options: c.applyKBDefault(opts),
	}
	var result IngestResult
	if err := c.do(ctx, http.MethodPost, "/v1/ingest/text", payload, &result); err != nil {
		return IngestResult{}, err
	}
	return result, nil
}

// IngestURLBatch submits multiple URLs in a single call.
// The server processes them in parallel. Returns one IngestResult per URL in the same order.
//
//	results, err := client.IngestURLBatch(ctx, []string{"https://a.com", "https://b.com"}, opts)
func (c *Client) IngestURLBatch(ctx context.Context, urls []string, opts IngestOpts) ([]IngestResult, error) {
	if len(urls) == 0 {
		return nil, &Error{Code: ErrCodeInvalidRequest, Message: "urls must not be empty"}
	}
	payload := struct {
		URLs    []string   `json:"urls"`
		Options IngestOpts `json:"options"`
	}{
		URLs:    urls,
		Options: c.applyKBDefault(opts),
	}
	var results []IngestResult
	if err := c.do(ctx, http.MethodPost, "/v1/ingest/url/batch", payload, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// -----------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------

// applyKBDefault fills KBID from the client's active KB if the caller left it empty.
func (c *Client) applyKBDefault(opts IngestOpts) IngestOpts {
	if opts.KBID == "" {
		opts.KBID = c.activeKBID
	}
	return opts
}

// createFormFile is like multipart.Writer.CreateFormFile but lets us set a custom MIME type.
func createFormFile(mw *multipart.Writer, fieldName, fileName, mimeType string) (io.Writer, error) {
	h := make(map[string][]string)
	h["Content-Disposition"] = []string{
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, fileName),
	}
	h["Content-Type"] = []string{mimeType}
	return mw.CreatePart(h)
}

// mimeFromExt returns a MIME type for the file extension.
func mimeFromExt(ext string) string {
	known := map[string]string{
		".pdf":  "application/pdf",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".txt":  "text/plain",
		".md":   "text/markdown",
		".csv":  "text/csv",
	}
	if m, ok := known[ext]; ok {
		return m
	}
	if m := mime.TypeByExtension(ext); m != "" {
		return m
	}
	return "application/octet-stream"
}
