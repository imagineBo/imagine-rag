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

// IngestFile uploads a local file to be parsed, chunked, embedded, and stored
// in a knowledge base.
//
// Supported formats: PDF, DOCX, PPTX, TXT, MD, CSV.
//
// The call returns as soon as the server accepts the file — processing happens
// asynchronously. Use [Client.WaitForFile] to block until the file is ready, or
// poll [Client.GetFileStatus] manually.
//
//	result, err := client.IngestFile(ctx, "./docs/manual.pdf", imagine.IngestOpts{})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Wait until the server finishes processing.
//	file, err := client.WaitForFile(ctx, result.FileID, 0)
func (c *Client) IngestFile(ctx context.Context, filePath string, opts IngestOpts) (IngestResult, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return IngestResult{}, fmt.Errorf("imagine: open file: %w", err)
	}
	defer f.Close()

	// Encode opts as JSON to include alongside the binary file data.
	optsJSON, err := json.Marshal(c.applyKBDefault(opts))
	if err != nil {
		return IngestResult{}, fmt.Errorf("imagine: marshal opts: %w", err)
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// Write the file field with the correct MIME type.
	mimeType := mimeFromExt(filepath.Ext(filePath))
	part, err := createFormFile(mw, "file", filepath.Base(filePath), mimeType)
	if err != nil {
		return IngestResult{}, fmt.Errorf("imagine: create form file: %w", err)
	}
	if _, err = io.Copy(part, f); err != nil {
		return IngestResult{}, fmt.Errorf("imagine: write file to form: %w", err)
	}

	// Write the options as a separate form field.
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

// IngestURL tells the server to fetch and process a single publicly accessible
// web page.
//
// The call returns immediately — fetching and processing happen asynchronously.
// Use [Client.WaitForFile] or [Client.GetFileStatus] to check progress.
//
//	result, err := client.IngestURL(ctx, "https://docs.acme.com/faq", imagine.IngestOpts{})
func (c *Client) IngestURL(ctx context.Context, url string, opts IngestOpts) (IngestResult, error) {
	if url == "" {
		return IngestResult{}, &Error{Code: ErrCodeInvalidRequest, Message: "url must not be empty"}
	}
	payload := struct {
		URL     string    `json:"url"`
		Options IngestOpts `json:"options"`
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

// IngestText sends raw string content directly to the knowledge base.
//
// name is a human-readable label shown in the admin panel (e.g. "Q3 release notes").
// Use this for dynamically generated content or content that does not live in a file.
//
//	result, err := client.IngestText(ctx,
//	    "Our return policy allows returns within 30 days.",
//	    "return-policy-v2",
//	    imagine.IngestOpts{},
//	)
func (c *Client) IngestText(ctx context.Context, content, name string, opts IngestOpts) (IngestResult, error) {
	if content == "" {
		return IngestResult{}, &Error{Code: ErrCodeInvalidRequest, Message: "content must not be empty"}
	}
	payload := struct {
		Content string    `json:"content"`
		Name    string    `json:"name"`
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

// IngestURLBatch submits multiple URLs in a single request.
//
// The server fetches and processes them in parallel. The returned slice has one
// [IngestResult] per URL in the same order as the input slice.
//
//	results, err := client.IngestURLBatch(ctx, []string{
//	    "https://docs.acme.com/page1",
//	    "https://docs.acme.com/page2",
//	    "https://docs.acme.com/page3",
//	}, imagine.IngestOpts{})
func (c *Client) IngestURLBatch(ctx context.Context, urls []string, opts IngestOpts) ([]IngestResult, error) {
	if len(urls) == 0 {
		return nil, &Error{Code: ErrCodeInvalidRequest, Message: "urls must not be empty"}
	}
	payload := struct {
		URLs    []string  `json:"urls"`
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
// Helpers
// -----------------------------------------------------------------------

// applyKBDefault copies opts and fills in KBID from the client's active KB
// when the caller left it empty. This keeps KB defaulting in one place.
func (c *Client) applyKBDefault(opts IngestOpts) IngestOpts {
	if opts.KBID == "" {
		opts.KBID = c.activeKBID
	}
	return opts
}

// createFormFile is like multipart.Writer.CreateFormFile but accepts a custom
// MIME type instead of always using application/octet-stream.
func createFormFile(mw *multipart.Writer, fieldName, fileName, mimeType string) (io.Writer, error) {
	h := make(map[string][]string)
	h["Content-Disposition"] = []string{
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, fileName),
	}
	h["Content-Type"] = []string{mimeType}
	return mw.CreatePart(h)
}

// mimeFromExt maps common file extensions to their MIME types.
// Falls back to mime.TypeByExtension from the OS, then application/octet-stream.
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
