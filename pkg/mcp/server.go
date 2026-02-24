package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/isitobservable/k8s-networking-mcp/pkg/tools"
	"github.com/isitobservable/k8s-networking-mcp/pkg/types"
)

type Server struct {
	mcpServer  *mcp.Server
	httpServer *http.Server
	registry   *tools.Registry

	mu              sync.Mutex
	registeredTools map[string]struct{} // tracks tools currently registered in mcpServer
}

func NewServer(registry *tools.Registry) *Server {
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "mcp-k8s-networking",
		Version: "1.0.0",
	}, nil)

	return &Server{
		mcpServer:       mcpServer,
		registry:        registry,
		registeredTools: make(map[string]struct{}),
	}
}

// SyncTools diffs the registry against what is currently registered in the MCP server,
// adding new tools and removing stale ones.
func (s *Server) SyncTools() {
	s.mu.Lock()
	defer s.mu.Unlock()

	registryTools := s.registry.List()

	// Build a set of tool names currently in the registry
	wanted := make(map[string]struct{}, len(registryTools))
	for _, t := range registryTools {
		wanted[t.Name()] = struct{}{}
	}

	// Remove tools that are registered but no longer in the registry
	var toRemove []string
	for name := range s.registeredTools {
		if _, ok := wanted[name]; !ok {
			toRemove = append(toRemove, name)
		}
	}
	if len(toRemove) > 0 {
		s.mcpServer.RemoveTools(toRemove...)
		for _, name := range toRemove {
			delete(s.registeredTools, name)
		}
		slog.Info("mcp: removed tools", "tools", toRemove)
	}

	// Add tools that are in the registry but not yet registered
	added := 0
	for _, t := range registryTools {
		if _, ok := s.registeredTools[t.Name()]; ok {
			continue
		}
		mcpTool := buildMCPTool(t)
		handler := buildHandler(t)
		s.mcpServer.AddTool(mcpTool, handler)
		s.registeredTools[t.Name()] = struct{}{}
		added++
	}

	slog.Info("mcp: synced tools", "total", len(s.registeredTools), "added", added, "removed", len(toRemove))
}

func (s *Server) Start(addr string) error {
	s.SyncTools()

	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return s.mcpServer
	}, nil)

	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	slog.Info("mcp: starting Streamable HTTP server", "addr", addr)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

func buildMCPTool(t tools.Tool) *mcp.Tool {
	schema := t.InputSchema()
	schemaJSON, _ := json.Marshal(schema)

	tool := &mcp.Tool{
		Name:        t.Name(),
		Description: t.Description(),
	}

	// Parse the JSON schema into the go-sdk's jsonschema.Schema type
	if err := json.Unmarshal(schemaJSON, &tool.InputSchema); err != nil {
		slog.Warn("mcp: failed to parse input schema", "tool", t.Name(), "error", err)
	}

	return tool
}

func buildHandler(t tools.Tool) mcp.ToolHandler {
	return func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Unmarshal arguments from the request
		var args map[string]interface{}
		if request.Params.Arguments != nil {
			if err := json.Unmarshal(request.Params.Arguments, &args); err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to parse arguments: %v", err)}},
					IsError: true,
				}, nil
			}
		}
		if args == nil {
			args = make(map[string]interface{})
		}

		result, err := t.Run(ctx, args)
		if err != nil {
			// Format MCPError consistently if available
			if mcpErr, ok := err.(*types.MCPError); ok {
				errJSON, _ := json.MarshalIndent(mcpErr, "", "  ")
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: string(errJSON)}},
					IsError: true,
				}, nil
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
				IsError: true,
			}, nil
		}

		// Apply compact/detail filtering if the response contains a ToolResult
		if result != nil {
			if tr, ok := result.Data.(*types.ToolResult); ok {
				detail := false
				if d, ok := args["detail"]; ok {
					if b, ok := d.(bool); ok {
						detail = b
					}
				}
				tr.Findings = types.FilterFindings(tr.Findings, detail)
			}
		}

		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to marshal result: %v", err)}},
				IsError: true,
			}, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(jsonBytes)}},
		}, nil
	}
}
