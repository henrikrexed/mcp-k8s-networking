package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/isitobservable/k8s-networking-mcp/pkg/config"
	"github.com/isitobservable/k8s-networking-mcp/pkg/discovery"
	"github.com/isitobservable/k8s-networking-mcp/pkg/k8s"
	mcpserver "github.com/isitobservable/k8s-networking-mcp/pkg/mcp"
	"github.com/isitobservable/k8s-networking-mcp/pkg/telemetry"
	"github.com/isitobservable/k8s-networking-mcp/pkg/tools"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}

	config.SetupLogging(cfg.LogLevel)

	slog.Info("starting mcp-k8s-networking server", "cluster", cfg.ClusterName, "port", cfg.Port)

	// Initialize OpenTelemetry tracer
	tracerShutdown, err := telemetry.InitTracer(context.Background(), cfg.ClusterName)
	if err != nil {
		slog.Error("failed to initialize tracer", "error", err)
		os.Exit(1)
	}

	// Initialize K8s clients
	clients, err := k8s.NewClients()
	if err != nil {
		slog.Error("failed to create K8s clients", "error", err)
		os.Exit(1)
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
	registry.Register(&tools.CheckKubeProxyHealthTool{BaseTool: base})
	registry.Register(&tools.ListIngressesTool{BaseTool: base})
	registry.Register(&tools.GetIngressTool{BaseTool: base})

	// Register log tools (always available)
	registry.Register(&tools.GetProxyLogsTool{BaseTool: base})
	registry.Register(&tools.GetGatewayLogsTool{BaseTool: base})
	registry.Register(&tools.GetInfraLogsTool{BaseTool: base})
	registry.Register(&tools.AnalyzeLogErrorsTool{BaseTool: base})

	// Create MCP server
	srv := mcpserver.NewServer(registry)

	// Gateway API tool names for conditional registration
	gatewayToolNames := []string{"list_gateways", "get_gateway", "list_httproutes", "get_httproute", "list_grpcroutes", "get_grpcroute", "list_referencegrants", "get_referencegrant", "scan_gateway_misconfigs", "check_gateway_conformance"}
	istioToolNames := []string{"list_istio_resources", "get_istio_resource", "check_sidecar_injection", "check_istio_mtls", "validate_istio_config"}

	// CRD discovery with onChange callback
	disc := discovery.New(clients.Discovery, clients.Dynamic, func(features discovery.Features) {

		// Gateway API tools
		if features.HasGatewayAPI {
			registry.Register(&tools.ListGatewaysTool{BaseTool: base})
			registry.Register(&tools.GetGatewayTool{BaseTool: base})
			registry.Register(&tools.ListHTTPRoutesTool{BaseTool: base})
			registry.Register(&tools.GetHTTPRouteTool{BaseTool: base})
			registry.Register(&tools.ListGRPCRoutesTool{BaseTool: base})
			registry.Register(&tools.GetGRPCRouteTool{BaseTool: base})
			registry.Register(&tools.ListReferenceGrantsTool{BaseTool: base})
			registry.Register(&tools.GetReferenceGrantTool{BaseTool: base})
			registry.Register(&tools.ScanGatewayMisconfigsTool{BaseTool: base})
			registry.Register(&tools.CheckGatewayConformanceTool{BaseTool: base})
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
			registry.Register(&tools.ValidateIstioConfigTool{BaseTool: base})
		} else {
			for _, name := range istioToolNames {
				registry.Unregister(name)
			}
		}

		// Re-sync tools with MCP server
		srv.SyncTools()
	})

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	disc.Start(ctx)

	// Health check endpoints
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})
	healthMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if !disc.IsReady() {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "not ready: initial CRD discovery pending")
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	// Start health check server on a separate port
	go func() {
		healthAddr := fmt.Sprintf(":%d", cfg.Port+1)
		slog.Info("health check server listening", "addr", healthAddr)
		if err := http.ListenAndServe(healthAddr, healthMux); err != nil && err != http.ErrServerClosed {
			slog.Error("health server error", "error", err)
		}
	}()

	// Start MCP Streamable HTTP server
	go func() {
		addr := fmt.Sprintf(":%d", cfg.Port)
		if err := srv.Start(addr); err != nil && err != http.ErrServerClosed {
			slog.Error("MCP server error", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("server ready", "port", cfg.Port)

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}

	// Flush pending OTel spans before exit
	if err := tracerShutdown(shutdownCtx); err != nil {
		slog.Error("tracer shutdown error", "error", err)
	}

	slog.Info("server stopped")
}
