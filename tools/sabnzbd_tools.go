package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jakenesler/navigatorr/sabnzbd"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// SABnzbd takes priority as a number. Names are accepted too so callers do not
// have to look the values up.
var sabPriorities = map[string]string{
	"default": "-100",
	"stop":    "-4",
	"paused":  "-2",
	"low":     "-1",
	"normal":  "0",
	"high":    "1",
	"force":   "2",
}

func registerSabnzbdTools(s *server.MCPServer, client *sabnzbd.Client, allowDestructive bool) {
	// sabnzbd_list_queue
	s.AddTool(
		mcp.NewTool("sabnzbd_list_queue",
			mcp.WithDescription("List active SABnzbd downloads with status, progress, and speed"),
			mcp.WithNumber("limit", mcp.Description("Maximum jobs to return")),
			mcp.WithNumber("start", mcp.Description("Offset into the queue")),
			mcp.WithString("category", mcp.Description("Only jobs in this category")),
			mcp.WithString("search", mcp.Description("Only jobs matching this term")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			queue, err := client.GetQueue(ctx,
				int(mcp.ParseFloat64(req, "start", 0)),
				int(mcp.ParseFloat64(req, "limit", 0)),
				mcp.ParseString(req, "category", ""),
				mcp.ParseString(req, "search", ""),
			)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to list queue: %v", err)), nil
			}
			data, _ := json.MarshalIndent(queue, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// sabnzbd_history
	s.AddTool(
		mcp.NewTool("sabnzbd_history",
			mcp.WithDescription("List finished and failed SABnzbd downloads. Use limit, since history can be very large"),
			mcp.WithNumber("limit", mcp.Description("Maximum entries to return")),
			mcp.WithNumber("start", mcp.Description("Offset into the history")),
			mcp.WithString("category", mcp.Description("Only entries in this category")),
			mcp.WithString("search", mcp.Description("Only entries matching this term")),
			mcp.WithBoolean("failed_only", mcp.Description("Only failed downloads")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			history, err := client.GetHistory(ctx,
				int(mcp.ParseFloat64(req, "start", 0)),
				int(mcp.ParseFloat64(req, "limit", 0)),
				mcp.ParseString(req, "category", ""),
				mcp.ParseString(req, "search", ""),
				mcp.ParseBoolean(req, "failed_only", false),
			)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get history: %v", err)), nil
			}
			data, _ := json.MarshalIndent(history, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// sabnzbd_add_nzb
	s.AddTool(
		mcp.NewTool("sabnzbd_add_nzb",
			mcp.WithDescription("Queue an NZB in SABnzbd by URL"),
			mcp.WithString("url", mcp.Required(), mcp.Description("Link to the NZB")),
			mcp.WithString("name", mcp.Description("Job name to use instead of the one in the NZB")),
			mcp.WithString("category", mcp.Description("Category to file the job under")),
			mcp.WithString("priority", mcp.Description("default, stop, paused, low, normal, high, or force")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			nzbURL := mcp.ParseString(req, "url", "")
			if nzbURL == "" {
				return mcp.NewToolResultError("url is required"), nil
			}

			priority := mcp.ParseString(req, "priority", "")
			if priority != "" {
				mapped, ok := sabPriorities[priority]
				if !ok {
					return mcp.NewToolResultError(fmt.Sprintf("unknown priority %q (use: default, stop, paused, low, normal, high, force)", priority)), nil
				}
				priority = mapped
			}

			ids, err := client.AddURL(ctx, nzbURL, mcp.ParseString(req, "name", ""), mcp.ParseString(req, "category", ""), priority)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to add NZB: %v", err)), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Queued as %v", ids)), nil
		},
	)

	// sabnzbd_manage_item
	s.AddTool(
		mcp.NewTool("sabnzbd_manage_item",
			mcp.WithDescription("Pause, resume, delete, reprioritise, or move a SABnzbd job by nzo_id"),
			mcp.WithString("action", mcp.Required(), mcp.Description("Action: pause, resume, delete, delete_files, priority, move")),
			mcp.WithString("nzo_id", mcp.Required(), mcp.Description("Job id, or \"all\" where SABnzbd accepts it")),
			mcp.WithString("value", mcp.Description("Priority name for priority, or target job id or queue position for move")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			action := mcp.ParseString(req, "action", "")
			nzoID := mcp.ParseString(req, "nzo_id", "")
			value := mcp.ParseString(req, "value", "")

			if nzoID == "" {
				return mcp.NewToolResultError("nzo_id is required"), nil
			}

			// SABnzbd deletes are GET requests carrying name=delete, so the
			// call_api DELETE guard never sees them. Gate them here instead.
			if (action == "delete" || action == "delete_files") && !allowDestructive {
				return mcp.NewToolResultError("Deleting is disabled. Set allow_destructive: true in config.yaml to enable."), nil
			}

			var (
				body []byte
				err  error
			)
			switch action {
			case "pause", "resume":
				body, err = client.QueueAction(ctx, action, nzoID, "")
			case "delete":
				body, err = client.QueueAction(ctx, "delete", nzoID, "")
			case "delete_files":
				body, err = client.Do(ctx, "queue", map[string]string{"name": "delete", "value": nzoID, "del_files": "1"})
			case "priority":
				mapped, ok := sabPriorities[value]
				if !ok {
					return mcp.NewToolResultError(fmt.Sprintf("unknown priority %q (use: default, stop, paused, low, normal, high, force)", value)), nil
				}
				body, err = client.QueueAction(ctx, "priority", nzoID, mapped)
			case "move":
				if value == "" {
					return mcp.NewToolResultError("move needs value set to a target job id or queue position"), nil
				}
				body, err = client.Move(ctx, nzoID, value)
			default:
				return mcp.NewToolResultError(fmt.Sprintf("unknown action %q (use: pause, resume, delete, delete_files, priority, move)", action)), nil
			}

			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("action %s failed: %v", action, err)), nil
			}
			return mcp.NewToolResultText(string(body)), nil
		},
	)

	// sabnzbd_status
	s.AddTool(
		mcp.NewTool("sabnzbd_status",
			mcp.WithDescription("SABnzbd version, speed, disk space, paused state, and warning count"),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			queue, err := client.GetQueue(ctx, 0, 1, "", "")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get status: %v", err)), nil
			}

			status := map[string]any{
				"version":     queue.Version,
				"paused":      queue.Paused,
				"status":      queue.Status,
				"speed":       queue.Speed,
				"speed_limit": queue.SpeedLimit,
				"size_left":   queue.SizeLeft,
				"time_left":   queue.TimeLeft,
				"disk_free":   queue.DiskSpace1Norm,
				"queued_jobs": queue.NoOfSlotsTotal,
				"warnings":    queue.HaveWarnings,
			}
			data, _ := json.MarshalIndent(status, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)
}
