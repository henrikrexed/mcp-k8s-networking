package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/isitobservable/k8s-networking-mcp/pkg/config"
	"github.com/isitobservable/k8s-networking-mcp/pkg/discovery"
	"github.com/isitobservable/k8s-networking-mcp/pkg/k8s"
	mcpserver "github.com/isitobservable/k8s-networking-mcp/pkg/mcp"
	"github.com/isitobservable/k8s-networking-mcp/pkg/tools"
)

func main() {
	cfg := config.Load()
	log.Printf("Starting mcp-k8s-networking server (cluster=%s, port=%d)", cfg.ClusterName, cfg.Port)

	// Initialize K8s clients
	clients, err := k8s.NewClients()
	if err != nil {
		log.Fatalf("Failed to create K8s clients: %v", err)
	}

	// Create tool registry
	registry := tools.NewRegistry()

	base := tools.BaseTool{Cfg: cfg, Clients: clients}

	// Register core K8s tools (always available)
	registry.Register(&tools.ListServicesTool{BaseTool: base})
	registry.Register(&tools.GetServiceTool{BaseTool: base})
	registry.Register(&tools.ListEndpointsTool{BaseTool: base})
	registry.Register(&tools.ListNetworkPoliciesTool{BaseTool: base})
	registry.Register(&tools.GetNetworkPolicyTool{BaseTool: base})
	registry.Register(&tools.CheckDNSTool{BaseTool: base})

	// Register log tools (always available)
	registry.Register(&tools.GetProxyLogsTool{BaseTool: base})
	registry.Register(&tools.GetGatewayLogsTool{BaseTool: base})
	registry.Register(&tools.GetInfraLogsTool{BaseTool: base})
	registry.Register(&tools.AnalyzeLogErrorsTool{BaseTool: base})

	// Create MCP server
	srv := mcpserver.NewServer(registry, cfg.Port)

	// Gateway API tool names for conditional registration
	gatewayToolNames := []string{"list_gateways", "get_gateway", "list_httproutes", "get_httproute"}
	istioToolNames := []string{"list_istio_resources", "get_istio_resource", "check_sidecar_injection", "check_istio_mtls"}

	// CRD discovery with onChange callback
	disc := discovery.New(clients.Discovery, func(features discovery.Features) {
		log.Printf("Discovery: GatewayAPI=%v, Istio=%v, Cilium=%v, Calico=%v, Linkerd=%v",
			features.HasGatewayAPI, features.HasIstio, features.HasCilium, features.HasCalico, features.HasLinkerd)

		// Gateway API tools
		if features.HasGatewayAPI {
			registry.Register(&tools.ListGatewaysTool{BaseTool: base})
			registry.Register(&tools.GetGatewayTool{BaseTool: base})
			registry.Register(&tools.ListHTTPRoutesTool{BaseTool: base})
			registry.Register(&tools.GetHTTPRouteTool{BaseTool: base})
		} else {
			for _, name := range gatewayToolNames {
				registry.Unregister(name)
			}
		}

		// Istio tools
		if features.HasIstio {
			registry.Register(&tools.ListIstioResourcesTool{BaseTool: base})
			registry.Register(&tools.GetIstioResourceTool{BaseTool: base})
			registry.Register(&tools.CheckSidecarInjectionTool{BaseTool: base})
			registry.Register(&tools.CheckIstioMTLSTool{BaseTool: base})
		} else {
			for _, name := range istioToolNames {
				registry.Unregister(name)
			}
		}

		// Re-sync tools with MCP server
		srv.SyncTools()
	})

	disc.Start()
	defer disc.Stop()

	// Health check endpoint
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	// Start health check server on a separate port
	go func() {
		healthAddr := fmt.Sprintf(":%d", cfg.Port+1)
		log.Printf("Health check server listening on %s", healthAddr)
		if err := http.ListenAndServe(healthAddr, healthMux); err != nil && err != http.ErrServerClosed {
			log.Printf("Health server error: %v", err)
		}
	}()

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start MCP SSE server
	go func() {
		addr := fmt.Sprintf(":%d", cfg.Port)
		if err := srv.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("MCP server error: %v", err)
		}
	}()

	log.Printf("Server ready on port %d", cfg.Port)

	<-ctx.Done()
	log.Println("Shutting down...")

	shutdownCtx := context.Background()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}

	log.Println("Server stopped")
}
