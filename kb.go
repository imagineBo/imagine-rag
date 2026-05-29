package imagine

import (
	"context"
	"net/http"
)

// CreateKB creates a new knowledge base and returns its metadata.
//
// A knowledge base is an isolated collection of document chunks. You typically
// create one KB per product, language, or customer. After creation, set it as
// the default on the client so you don't have to pass KBID on every call:
//
//	kb, err := client.CreateKB(ctx, "Product Docs", imagine.KBOpts{
//	    Description: "Public documentation for Acme v2",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	client.SetActiveKB(kb.KBID) // all future ingest/query calls use this KB
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

// ListKBs returns all knowledge bases belonging to this tenant's API key.
//
// Use this to let your users choose which knowledge base to query, or to find
// an existing KB by name before deciding whether to create a new one.
//
//	kbs, err := client.ListKBs(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, kb := range kbs {
//	    fmt.Printf("%s  %s  (%d files)\n", kb.KBID, kb.Name, kb.FileCount)
//	}
func (c *Client) ListKBs(ctx context.Context) ([]KnowledgeBase, error) {
	var kbs []KnowledgeBase
	if err := c.do(ctx, http.MethodGet, "/v1/kb", nil, &kbs); err != nil {
		return nil, err
	}
	return kbs, nil
}

// DeleteKB permanently deletes a knowledge base along with all its files and
// indexed chunks. This is irreversible — deleted content cannot be recovered.
//
// After deletion, any query that targets this KB will return a not-found error.
//
//	err := client.DeleteKB(ctx, kb.KBID)
//	if err != nil && !imagine.IsNotFound(err) {
//	    log.Fatal(err)
//	}
func (c *Client) DeleteKB(ctx context.Context, kbID string) error {
	if kbID == "" {
		return &Error{Code: ErrCodeInvalidRequest, Message: "kbID must not be empty"}
	}
	return c.do(ctx, http.MethodDelete, "/v1/kb/"+kbID, nil, nil)
}
