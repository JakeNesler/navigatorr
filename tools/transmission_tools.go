package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jakenesler/navigatorr/transmission"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTransmissionTools(s *server.MCPServer, client *transmission.Client) {
	// transmission_list_torrents
	s.AddTool(
		mcp.NewTool("transmission_list_torrents",
			mcp.WithDescription("List all torrents with their status, progress, and download info"),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			torrents, err := client.TorrentGet(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to list torrents: %v", err)), nil
			}
			data, _ := json.MarshalIndent(torrents, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// transmission_add_torrent
	s.AddTool(
		mcp.NewTool("transmission_add_torrent",
			mcp.WithDescription("Add a torrent by magnet link or URL"),
			mcp.WithString("url", mcp.Required(), mcp.Description("Magnet link or torrent URL")),
			mcp.WithString("download_dir", mcp.Description("Download directory (optional)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			url := mcp.ParseString(req, "url", "")
			downloadDir := mcp.ParseString(req, "download_dir", "")

			info, err := client.TorrentAdd(ctx, url, downloadDir)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to add torrent: %v", err)), nil
			}
			data, _ := json.MarshalIndent(info, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// transmission_manage_torrent
	s.AddTool(
		mcp.NewTool("transmission_manage_torrent",
			mcp.WithDescription("Manage a torrent: start, stop, remove, or verify"),
			mcp.WithString("action", mcp.Required(), mcp.Description("Action: start, stop, remove, remove_data, verify")),
			mcp.WithString("ids", mcp.Required(), mcp.Description("Comma-separated torrent IDs (e.g. \"1,2,3\")")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			action := mcp.ParseString(req, "action", "")
			idsStr := mcp.ParseString(req, "ids", "")

			ids, err := parseIDs(idsStr)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			switch action {
			case "start":
				err = client.TorrentStart(ctx, ids)
			case "stop":
				err = client.TorrentStop(ctx, ids)
			case "remove":
				err = client.TorrentRemove(ctx, ids, false)
			case "remove_data":
				err = client.TorrentRemove(ctx, ids, true)
			case "verify":
				err = client.TorrentVerify(ctx, ids)
			default:
				return mcp.NewToolResultError(fmt.Sprintf("unknown action %q (use: start, stop, remove, remove_data, verify)", action)), nil
			}

			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("action %s failed: %v", action, err)), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Successfully executed %s on torrent(s) %v", action, ids)), nil
		},
	)

	// transmission_free_space
	s.AddTool(
		mcp.NewTool("transmission_free_space",
			mcp.WithDescription("Check free disk space at a given path"),
			mcp.WithString("path", mcp.Description("Path to check (defaults to /data/downloads)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			path := mcp.ParseString(req, "path", "/data/downloads")

			bytes, err := client.FreeSpace(ctx, path)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to check free space: %v", err)), nil
			}

			gb := float64(bytes) / (1024 * 1024 * 1024)
			return mcp.NewToolResultText(fmt.Sprintf("Free space at %s: %.2f GB (%d bytes)", path, gb, bytes)), nil
		},
	)
}

func parseIDs(s string) ([]int, error) {
	if s == "" {
		return nil, fmt.Errorf("ids is required")
	}

	var ids []int
	// Try JSON array first
	if err := json.Unmarshal([]byte(s), &ids); err == nil {
		return ids, nil
	}

	// Parse comma-separated
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		var id int
		if _, err := fmt.Sscanf(p, "%d", &id); err != nil {
			return nil, fmt.Errorf("invalid ID %q", p)
		}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("no valid IDs provided")
	}

	return ids, nil
}
