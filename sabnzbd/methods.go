package sabnzbd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

// GetQueue returns the download queue.
func (c *Client) GetQueue(ctx context.Context, start, limit int, category, search string) (*Queue, error) {
	data, err := c.Do(ctx, "queue", map[string]string{
		"start":  positive(start),
		"limit":  positive(limit),
		"cat":    category,
		"search": search,
	})
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Queue Queue `json:"queue"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("decoding queue: %w", err)
	}
	return &wrapper.Queue, nil
}

// GetHistory returns finished and failed jobs. SABnzbd applies its own default
// page size when limit is zero, so callers get a page rather than everything.
func (c *Client) GetHistory(ctx context.Context, start, limit int, category, search string, failedOnly bool) (*History, error) {
	params := map[string]string{
		"start":  positive(start),
		"limit":  positive(limit),
		"cat":    category,
		"search": search,
	}
	if failedOnly {
		params["failed_only"] = "1"
	}

	data, err := c.Do(ctx, "history", params)
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		History History `json:"history"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("decoding history: %w", err)
	}
	return &wrapper.History, nil
}

// AddURL queues an NZB by URL and returns the resulting nzo_ids.
func (c *Client) AddURL(ctx context.Context, nzbURL, name, category, priority string) ([]string, error) {
	data, err := c.Do(ctx, "addurl", map[string]string{
		"name":     nzbURL,
		"nzbname":  name,
		"cat":      category,
		"priority": priority,
	})
	if err != nil {
		return nil, err
	}

	var result struct {
		NzoIDs []string `json:"nzo_ids"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decoding addurl response: %w", err)
	}
	return result.NzoIDs, nil
}

// QueueAction runs a name= action against one or more queue jobs. value2 carries
// the extra argument that priority takes and the other actions do not.
func (c *Client) QueueAction(ctx context.Context, action, nzoID, value2 string) ([]byte, error) {
	params := map[string]string{
		"name":   action,
		"value":  nzoID,
		"value2": value2,
	}
	return c.Do(ctx, "queue", params)
}

// DeleteHistory removes history entries. nzoID also accepts SABnzbd's "all",
// "failed" and "completed" selectors.
func (c *Client) DeleteHistory(ctx context.Context, nzoID string, deleteFiles bool) ([]byte, error) {
	params := map[string]string{
		"name":  "delete",
		"value": nzoID,
	}
	if deleteFiles {
		params["del_files"] = "1"
	}
	return c.Do(ctx, "history", params)
}

// Move puts a job above another job, or at a queue position when target is a number.
func (c *Client) Move(ctx context.Context, nzoID, target string) ([]byte, error) {
	return c.Do(ctx, "switch", map[string]string{
		"value":  nzoID,
		"value2": target,
	})
}

func positive(n int) string {
	if n <= 0 {
		return ""
	}
	return strconv.Itoa(n)
}
