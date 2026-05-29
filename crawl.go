package imagine

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// CrawlURL starts a crawl job from seedURL and returns immediately with a jobID.
// The server fetches pages, chunks them, and adds them to the knowledge base.
// Use GetCrawlStatus or WaitForCrawl to track progress.
//
//	job, err := client.CrawlURL(ctx, "https://docs.acme.com", imagine.CrawlOpts{
//	    MaxPages: 200,
//	    MaxDepth: 4,
//	    SameDomain: true,
//	})
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

// GetCrawlStatus returns the current state of a crawl job.
//
//	job, err := client.GetCrawlStatus(ctx, jobID)
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

// WaitForCrawl polls GetCrawlStatus every interval until the job reaches a
// terminal state (completed, failed, or cancelled), then returns the final job.
// Pass interval = 0 to use the default of 3 seconds.
// The context controls the overall deadline — cancel it to stop waiting early.
//
//	job, err := client.WaitForCrawl(ctx, jobID, 5*time.Second)
//	if job.Status == "completed" {
//	    fmt.Println("crawl done, pages:", job.PagesDone)
//	}
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
// Already-ingested pages remain in the knowledge base.
//
//	err := client.CancelCrawl(ctx, jobID)
func (c *Client) CancelCrawl(ctx context.Context, jobID string) error {
	if jobID == "" {
		return &Error{Code: ErrCodeInvalidRequest, Message: "jobID must not be empty"}
	}
	return c.do(ctx, http.MethodDelete, "/v1/crawl/"+jobID, nil, nil)
}
