package qbit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ListTorrents returns all torrents.
func (c *Client) ListTorrents(ctx context.Context) ([]TorrentInfo, error) {
	data, err := c.do(ctx, "GET", "/api/v2/torrents/info", nil)
	if err != nil {
		return nil, err
	}

	var torrents []TorrentInfo
	if err := json.Unmarshal(data, &torrents); err != nil {
		return nil, fmt.Errorf("decoding torrents: %w", err)
	}
	return torrents, nil
}

// AddTorrent adds a torrent by magnet link or URL.
func (c *Client) AddTorrent(ctx context.Context, urls string, savePath string) error {
	form := url.Values{
		"urls": {urls},
	}
	if savePath != "" {
		form.Set("savepath", savePath)
	}

	data, err := c.do(ctx, "POST", "/api/v2/torrents/add", form)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(data)) == "Fails." {
		return fmt.Errorf("qBittorrent rejected the torrent")
	}
	return nil
}

// PauseTorrents pauses torrents by hash.
func (c *Client) PauseTorrents(ctx context.Context, hashes []string) error {
	return c.setTorrentState(ctx, hashes, "stop", "pause")
}

// ResumeTorrents resumes torrents by hash.
func (c *Client) ResumeTorrents(ctx context.Context, hashes []string) error {
	return c.setTorrentState(ctx, hashes, "start", "resume")
}

func (c *Client) setTorrentState(ctx context.Context, hashes []string, action, legacyAction string) error {
	form := url.Values{
		"hashes": {strings.Join(hashes, "|")},
	}
	_, err := c.do(ctx, "POST", "/api/v2/torrents/"+action, form)
	// qBittorrent 5 renamed pause/resume to stop/start. Only a missing
	// endpoint warrants retrying the older name, never an execution failure.
	var status *httpStatusError
	if errors.As(err, &status) && status.code == http.StatusNotFound {
		_, err = c.do(ctx, "POST", "/api/v2/torrents/"+legacyAction, form)
	}
	return err
}

// DeleteTorrents deletes torrents by hash, optionally deleting files.
func (c *Client) DeleteTorrents(ctx context.Context, hashes []string, deleteFiles bool) error {
	form := url.Values{
		"hashes":      {strings.Join(hashes, "|")},
		"deleteFiles": {fmt.Sprintf("%t", deleteFiles)},
	}
	_, err := c.do(ctx, "POST", "/api/v2/torrents/delete", form)
	return err
}

// GetTransferInfo returns global transfer statistics.
func (c *Client) GetTransferInfo(ctx context.Context) (*TransferInfo, error) {
	data, err := c.do(ctx, "GET", "/api/v2/transfer/info", nil)
	if err != nil {
		return nil, err
	}

	var info TransferInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("decoding transfer info: %w", err)
	}
	return &info, nil
}
