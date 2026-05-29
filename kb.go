package imagine

import (
	"context"
	"net/http"
)

// CreateKB creates a new knowledge base and returns it.
//
//	kb, err := client.CreateKB(ctx, "Product Docs", imagine.KBOpts{
//	    Description: "All product documentation",
//	})
//	client.SetActiveKB(kb.KBID) // set as default for future calls
func (c *Client) CreateKB(ctx context.Context, name string, opts KBOpts) (KnowledgeBase, error) {
	if name == "" {
		return KnowledgeBase{}, &Error{Code: ErrCodeInvalidRequest, Message: "name must not be empty"}
	}
	payload := struct {
		Name string `json:"name"`
		KBOpts
	}{
		Name:   name,
		KBOpts: opts,
	}
	var kb KnowledgeBase
	if err := c.do(ctx, http.MethodPost, "/v1/kb", payload, &kb); err != nil {
		return KnowledgeBase{}, err
	}
	return kb, nil
}

// ListKBs returns all knowledge bases belonging to this tenant.
//
//	kbs, err := client.ListKBs(ctx)
//	for _, kb := range kbs {
//	    fmt.Println(kb.KBID, kb.Name, kb.FileCount)
//	}
func (c *Client) ListKBs(ctx context.Context) ([]KnowledgeBase, error) {
	var kbs []KnowledgeBase
	if err := c.do(ctx, http.MethodGet, "/v1/kb", nil, &kbs); err != nil {
		return nil, err
	}
	return kbs, nil
}

// DeleteKB deletes a knowledge base and all its files and chunks.
// This is irreversible.
//
//	err := client.DeleteKB(ctx, kbID)
func (c *Client) DeleteKB(ctx context.Context, kbID string) error {
	if kbID == "" {
		return &Error{Code: ErrCodeInvalidRequest, Message: "kbID must not be empty"}
	}
	return c.do(ctx, http.MethodDelete, "/v1/kb/"+kbID, nil, nil)
}
