package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/isitobservable/k8s-networking-mcp/pkg/tools"
)

type Server struct {
	mcpServer *server.MCPServer
	sseServer *server.SSEServer
	registry  *tools.Registry
}

func NewServer(registry *tools.Registry, port int) *Server {
	mcpServer := server.NewMCPServer(
		"mcp-k8s-networking",
		"1.0.0",
		server.WithRecovery(),
	)

	sseServer := server.NewSSEServer(mcpServer,
		server.WithBaseURL(fmt.Sprintf("http://localhost:%d", port)),
	)

	s := &Server{
		mcpServer: mcpServer,
		sseServer: sseServer,
		registry:  registry,
	}

	return s
}

func (s *Server) SyncTools() {
	// Clear existing tools and re-add from registry
	for _, t := range s.registry.List() {
		tool := t
		mcpTool := buildMCPTool(tool)
		handler := buildHandler(tool)
		s.mcpServer.AddTool(mcpTool, handler)
	}
	log.Printf("mcp: synced %d tools", len(s.registry.List()))
}

func (s *Server) Start(addr string) error {
	s.SyncTools()
	log.Printf("mcp: starting SSE server on %s", addr)
	return s.sseServer.Start(addr)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.sseServer.Shutdown(ctx)
}

func buildMCPTool(t tools.Tool) mcp.Tool {
	schema := t.InputSchema()
	schemaJSON, _ := json.Marshal(schema)
	return mcp.NewToolWithRawSchema(t.Name(), t.Description(), schemaJSON)
}

func buildHandler(t tools.Tool) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := request.GetArguments()

		result, err := t.Run(ctx, args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}
