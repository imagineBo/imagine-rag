package imagine

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// CrawlURL starts a crawl job that spiders a website and ingests each page
// into a knowledge base as a separate document.
//
// The call returns immediately with a [CrawlJob] containing the JobID — the
// crawl runs asynchronously on the server. Use [Client.WaitForCrawl] to block
// until it finishes, or poll [Client.GetCrawlStatus] manually.
//
//	job, err := client.CrawlURL(ctx, "https://docs.acme.com", imagine.CrawlOpts{
//	    MaxPages:   200,
//	    MaxDepth:   4,
//	    SameDomain: true, // stay on docs.acme.com
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("crawl started:", job.JobID)
func (c *Client) CrawlURL(ctx context.Context, seedURL string, opts CrawlOpts) (CrawlJob, error) {
	if seedURL == "" {
		return CrawlJob{}, &Error{Code: ErrCodeInvalidRequest, Message: "seedURL must not be empty"}
	}
	if opts.KBID == "" {
		opts.KBID = c.activeKBID
	}
	payload := struct {
		SeedURL string   `json:"seed_url"`
		Options CrawlOpts `json:"options"`
	}{
		SeedURL: seedURL,
		Options: opts,
	}
	var job CrawlJob
	if err := c.do(ctx, http.MethodPost, "/v1/crawl", payload, &job); err != nil {
		return CrawlJob{}, err
	}
	return job, nil
}

// GetCrawlStatus fetches the current state of a crawl job without blocking.
//
// Check [CrawlJob.Status] to see whether the crawl is still running or has
// reached a terminal state ("completed", "failed", or "cancelled").
//
//	job, err := client.GetCrawlStatus(ctx, jobID)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("pages done: %d / %d\n", job.PagesDone, job.PagesTotal)
func (c *Client) GetCrawlStatus(ctx context.Context, jobID string) (CrawlJob, error) {
	if jobID == "" {
		return CrawlJob{}, &Error{Code: ErrCodeInvalidRequest, Message: "jobID must not be empty"}
	}
	var job CrawlJob
	if err := c.do(ctx, http.MethodGet, "/v1/crawl/"+jobID, nil, &job); err != nil {
		return CrawlJob{}, err
	}
	return job, nil
}

// WaitForCrawl polls [Client.GetCrawlStatus] at the given interval until the
// crawl reaches a terminal state ("completed", "failed", or "cancelled"), then
// returns the final [CrawlJob].
//
// Pass interval = 0 to use the default of 3 seconds.
// Cancel the context to stop waiting early — the crawl itself keeps running on
// the server; call [Client.CancelCrawl] to stop it.
//
//	// Wait up to 10 minutes, polling every 5 seconds.
//	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
//	defer cancel()
//
//	job, err := client.WaitForCrawl(ctx, jobID, 5*time.Second)
//	if err != nil {
//	    log.Fatal(err) // context deadline or API error
//	}
//	fmt.Printf("crawl %s — %d pages ingested\n", job.Status, job.PagesDone)
func (c *Client) WaitForCrawl(ctx context.Context, jobID string, interval time.Duration) (CrawlJob, error) {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	for {
		job, err := c.GetCrawlStatus(ctx, jobID)
		if err != nil {
			return CrawlJob{}, err
		}
		switch job.Status {
		case "completed", "failed", "cancelled":
			return job, nil
		}
		select {
		case <-ctx.Done():
			return CrawlJob{}, fmt.Errorf("imagine: WaitForCrawl cancelled: %w", ctx.Err())
		case <-time.After(interval):
		}
	}
}

// CancelCrawl stops a running crawl job.
//
// Pages that have already been fetched and ingested remain in the knowledge
// base. Only the remaining queued pages are discarded.
//
//	err := client.CancelCrawl(ctx, jobID)
func (c *Client) CancelCrawl(ctx context.Context, jobID string) error {
	if jobID == "" {
		return &Error{Code: ErrCodeInvalidRequest, Message: "jobID must not be empty"}
	}
	return c.do(ctx, http.MethodDelete, "/v1/crawl/"+jobID, nil, nil)
}
