package imagine

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// ListFiles returns all files in the specified knowledge base.
//
// Pass an empty kbID to list files across all knowledge bases for this tenant.
// If kbID is empty and a default KB is set on the client ([Client.SetActiveKB]),
// only that KB's files are returned.
//
//	files, err := client.ListFiles(ctx, "kb_abc123")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, f := range files {
//	    fmt.Printf("%s  %-12s  %s\n", f.FileID, f.Status, f.Name)
//	}
func (c *Client) ListFiles(ctx context.Context, kbID string) ([]File, error) {
	path := "/v1/files"
	if kbID != "" {
		path += "?kb_id=" + kbID
	} else if c.activeKBID != "" {
		path += "?kb_id=" + c.activeKBID
	}
	var files []File
	if err := c.do(ctx, http.MethodGet, path, nil, &files); err != nil {
		return nil, err
	}
	return files, nil
}

// GetFileStatus fetches the current state of an ingested file.
//
// The [File.Status] field transitions from "processing" → "ready" (success) or
// "processing" → "failed" (error). Use [Client.WaitForFile] to block until the
// transition happens.
//
//	file, err := client.GetFileStatus(ctx, fileID)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("status:", file.Status)
func (c *Client) GetFileStatus(ctx context.Context, fileID string) (File, error) {
	if fileID == "" {
		return File{}, &Error{Code: ErrCodeInvalidRequest, Message: "fileID must not be empty"}
	}
	var file File
	if err := c.do(ctx, http.MethodGet, "/v1/files/"+fileID, nil, &file); err != nil {
		return File{}, err
	}
	return file, nil
}

// WaitForFile polls [Client.GetFileStatus] at the given interval until the file
// reaches a terminal state ("ready" or "failed"), then returns the final [File].
//
// Pass interval = 0 to use the default of 2 seconds.
// Cancel the context to stop waiting early.
//
//	// Ingest and wait.
//	result, _ := client.IngestFile(ctx, "./manual.pdf", imagine.IngestOpts{})
//
//	file, err := client.WaitForFile(ctx, result.FileID, 0)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if file.Status == "failed" {
//	    log.Fatal("processing failed — check the dashboard for details")
//	}
//	fmt.Println("file is ready:", file.Name)
func (c *Client) WaitForFile(ctx context.Context, fileID string, interval time.Duration) (File, error) {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	for {
		file, err := c.GetFileStatus(ctx, fileID)
		if err != nil {
			return File{}, err
		}
		switch file.Status {
		case "ready", "failed":
			return file, nil
		}
		select {
		case <-ctx.Done():
			return File{}, fmt.Errorf("imagine: WaitForFile cancelled: %w", ctx.Err())
		case <-time.After(interval):
		}
	}
}

// DeleteFile permanently removes a file and all its indexed chunks from the
// knowledge base. This is irreversible.
//
// After deletion, the file's content is no longer returned in query results.
//
//	err := client.DeleteFile(ctx, fileID)
//	if err != nil && !imagine.IsNotFound(err) {
//	    log.Fatal(err)
//	}
func (c *Client) DeleteFile(ctx context.Context, fileID string) error {
	if fileID == "" {
		return &Error{Code: ErrCodeInvalidRequest, Message: "fileID must not be empty"}
	}
	return c.do(ctx, http.MethodDelete, "/v1/files/"+fileID, nil, nil)
}
