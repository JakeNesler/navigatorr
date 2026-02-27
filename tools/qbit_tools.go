package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jakenesler/navigatorr/qbit"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerQbitTools(s *server.MCPServer, client *qbit.Client) {
	// qbit_list_torrents
	s.AddTool(
		mcp.NewTool("qbit_list_torrents",
			mcp.WithDescription("List all torrents in qBittorrent with status, progress, and speed info"),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			torrents, err := client.ListTorrents(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to list torrents: %v", err)), nil
			}
			data, _ := json.MarshalIndent(torrents, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// qbit_add_torrent
	s.AddTool(
		mcp.NewTool("qbit_add_torrent",
			mcp.WithDescription("Add a torrent to qBittorrent by magnet link or URL"),
			mcp.WithString("url", mcp.Required(), mcp.Description("Magnet link or torrent URL")),
			mcp.WithString("save_path", mcp.Description("Download save path (optional)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			url := mcp.ParseString(req, "url", "")
			savePath := mcp.ParseString(req, "save_path", "")

			if err := client.AddTorrent(ctx, url, savePath); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to add torrent: %v", err)), nil
			}
			return mcp.NewToolResultText("Torrent added successfully"), nil
		},
	)

	// qbit_manage_torrent
	s.AddTool(
		mcp.NewTool("qbit_manage_torrent",
			mcp.WithDescription("Manage qBittorrent torrents: pause, resume, or delete by hash"),
			mcp.WithString("action", mcp.Required(), mcp.Description("Action: pause, resume, delete, delete_files")),
			mcp.WithString("hashes", mcp.Required(), mcp.Description("Comma-separated torrent hashes (or \"all\" for all torrents)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			action := mcp.ParseString(req, "action", "")
			hashesStr := mcp.ParseString(req, "hashes", "")

			hashes := parseHashes(hashesStr)
			if len(hashes) == 0 {
				return mcp.NewToolResultError("hashes is required"), nil
			}

			var err error
			switch action {
			case "pause":
				err = client.PauseTorrents(ctx, hashes)
			case "resume":
				err = client.ResumeTorrents(ctx, hashes)
			case "delete":
				err = client.DeleteTorrents(ctx, hashes, false)
			case "delete_files":
				err = client.DeleteTorrents(ctx, hashes, true)
			default:
				return mcp.NewToolResultError(fmt.Sprintf("unknown action %q (use: pause, resume, delete, delete_files)", action)), nil
			}

			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("action %s failed: %v", action, err)), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Successfully executed %s on torrent(s)", action)), nil
		},
	)

	// qbit_transfer_info
	s.AddTool(
		mcp.NewTool("qbit_transfer_info",
			mcp.WithDescription("Get qBittorrent global transfer speed and statistics"),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			info, err := client.GetTransferInfo(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get transfer info: %v", err)), nil
			}
			data, _ := json.MarshalIndent(info, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)
}

func parseHashes(s string) []string {
	if s == "" {
		return nil
	}
	if s == "all" {
		return []string{"all"}
	}
	var hashes []string
	for _, h := range strings.Split(s, ",") {
		h = strings.TrimSpace(h)
		if h != "" {
			hashes = append(hashes, h)
		}
	}
	return hashes
}
