package imagine

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// ListFiles returns all files in a knowledge base.
// Pass kbID = "" to list files across all knowledge bases for this tenant.
//
//	files, err := client.ListFiles(ctx, "kb_abc123")
//	for _, f := range files {
//	    fmt.Println(f.FileID, f.Name, f.Status)
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

// GetFileStatus returns the current state of an ingested file.
//
//	file, err := client.GetFileStatus(ctx, fileID)
//	fmt.Println("status:", file.Status) // "processing" | "ready" | "failed"
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

// WaitForFile polls GetFileStatus every interval until the file reaches a
// terminal state (ready or failed), then returns the final file record.
// Pass interval = 0 to use the default of 2 seconds.
// The context controls the overall deadline.
//
//	file, err := client.WaitForFile(ctx, fileID, 0)
//	if file.Status == "ready" {
//	    fmt.Println("file is ready to query")
//	}
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

// DeleteFile removes a file and all its chunks from the knowledge base.
//
//	err := client.DeleteFile(ctx, fileID)
func (c *Client) DeleteFile(ctx context.Context, fileID string) error {
	if fileID == "" {
		return &Error{Code: ErrCodeInvalidRequest, Message: "fileID must not be empty"}
	}
	return c.do(ctx, http.MethodDelete, "/v1/files/"+fileID, nil, nil)
}
