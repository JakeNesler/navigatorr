package transmission

import (
	"context"
	"encoding/json"
	"fmt"
)

var torrentFields = []string{
	"id", "name", "status", "percentDone", "totalSize",
	"downloadedEver", "uploadedEver", "rateDownload", "rateUpload",
	"eta", "error", "errorString", "downloadDir",
}

// TorrentGet returns a list of torrents.
func (c *Client) TorrentGet(ctx context.Context) ([]TorrentInfo, error) {
	resp, err := c.call(ctx, "torrent-get", map[string]any{
		"fields": torrentFields,
	})
	if err != nil {
		return nil, err
	}

	return parseTorrents(resp)
}

// TorrentAdd adds a torrent by magnet link or URL.
func (c *Client) TorrentAdd(ctx context.Context, magnetOrURL, downloadDir string) (*TorrentInfo, error) {
	args := map[string]any{
		"filename": magnetOrURL,
	}
	if downloadDir != "" {
		args["download-dir"] = downloadDir
	}

	resp, err := c.call(ctx, "torrent-add", args)
	if err != nil {
		return nil, err
	}

	// Response has "torrent-added" or "torrent-duplicate"
	for _, key := range []string{"torrent-added", "torrent-duplicate"} {
		if raw, ok := resp.Arguments[key]; ok {
			data, _ := json.Marshal(raw)
			var info TorrentInfo
			if err := json.Unmarshal(data, &info); err == nil {
				info.StatusText = StatusName(info.Status)
				return &info, nil
			}
		}
	}

	return nil, fmt.Errorf("unexpected response from torrent-add")
}

// TorrentStart starts torrents by ID.
func (c *Client) TorrentStart(ctx context.Context, ids []int) error {
	_, err := c.call(ctx, "torrent-start", map[string]any{"ids": ids})
	return err
}

// TorrentStop stops torrents by ID.
func (c *Client) TorrentStop(ctx context.Context, ids []int) error {
	_, err := c.call(ctx, "torrent-stop", map[string]any{"ids": ids})
	return err
}

// TorrentRemove removes torrents by ID.
func (c *Client) TorrentRemove(ctx context.Context, ids []int, deleteData bool) error {
	_, err := c.call(ctx, "torrent-remove", map[string]any{
		"ids":               ids,
		"delete-local-data": deleteData,
	})
	return err
}

// TorrentVerify verifies torrents by ID.
func (c *Client) TorrentVerify(ctx context.Context, ids []int) error {
	_, err := c.call(ctx, "torrent-verify", map[string]any{"ids": ids})
	return err
}

// FreeSpace returns free space at a given path.
func (c *Client) FreeSpace(ctx context.Context, path string) (int64, error) {
	resp, err := c.call(ctx, "free-space", map[string]any{"path": path})
	if err != nil {
		return 0, err
	}

	if sizeBytes, ok := resp.Arguments["size-bytes"]; ok {
		switch v := sizeBytes.(type) {
		case float64:
			return int64(v), nil
		case json.Number:
			n, _ := v.Int64()
			return n, nil
		}
	}

	return 0, fmt.Errorf("unexpected free-space response")
}

// SessionStats returns session statistics.
func (c *Client) SessionStats(ctx context.Context) (map[string]any, error) {
	resp, err := c.call(ctx, "session-stats", nil)
	if err != nil {
		return nil, err
	}
	return resp.Arguments, nil
}

func parseTorrents(resp *rpcResponse) ([]TorrentInfo, error) {
	raw, ok := resp.Arguments["torrents"]
	if !ok {
		return nil, nil
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshaling torrents: %w", err)
	}

	// The API returns camelCase fields; we need to manually map
	var rawTorrents []map[string]any
	if err := json.Unmarshal(data, &rawTorrents); err != nil {
		return nil, fmt.Errorf("parsing torrents: %w", err)
	}

	torrents := make([]TorrentInfo, 0, len(rawTorrents))
	for _, rt := range rawTorrents {
		t := TorrentInfo{
			ID:            intVal(rt, "id"),
			Name:          strVal(rt, "name"),
			Status:        intVal(rt, "status"),
			PercentDone:   floatVal(rt, "percentDone"),
			TotalSize:     int64Val(rt, "totalSize"),
			DownloadedEver: int64Val(rt, "downloadedEver"),
			UploadedEver:  int64Val(rt, "uploadedEver"),
			RateDownload:  int64Val(rt, "rateDownload"),
			RateUpload:    int64Val(rt, "rateUpload"),
			ETA:           intVal(rt, "eta"),
			Error:         intVal(rt, "error"),
			ErrorString:   strVal(rt, "errorString"),
			DownloadDir:   strVal(rt, "downloadDir"),
		}
		t.StatusText = StatusName(t.Status)
		torrents = append(torrents, t)
	}

	return torrents, nil
}

func intVal(m map[string]any, key string) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case json.Number:
			i, _ := n.Int64()
			return int(i)
		}
	}
	return 0
}

func int64Val(m map[string]any, key string) int64 {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int64(n)
		case json.Number:
			i, _ := n.Int64()
			return i
		}
	}
	return 0
}

func floatVal(m map[string]any, key string) float64 {
	if v, ok := m[key]; ok {
		if n, ok := v.(float64); ok {
			return n
		}
	}
	return 0
}

func strVal(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
