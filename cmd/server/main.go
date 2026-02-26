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
	"github.com/isitobservable/k8s-networking-mcp/pkg/probes"
	"github.com/isitobservable/k8s-networking-mcp/pkg/skills"
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

	// Initialize OpenTelemetry (traces + metrics + logs)
	otelResult, err := telemetry.Init(context.Background(), cfg.ClusterName)
	if err != nil {
		slog.Error("failed to initialize telemetry", "error", err)
		os.Exit(1)
	}

	// Replace default slog handler with OTel-bridged handler for trace correlation
	slog.SetDefault(slog.New(otelResult.SlogHandler))

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

	// Initialize probe manager and register probe tools (always available)
	probeMgr := probes.NewManager(context.Background(), cfg, clients)
	registry.Register(&tools.ProbeConnectivityTool{BaseTool: base, ProbeManager: probeMgr})
	registry.Register(&tools.ProbeDNSTool{BaseTool: base, ProbeManager: probeMgr})
	registry.Register(&tools.ProbeHTTPTool{BaseTool: base, ProbeManager: probeMgr})

	// Create skills registry
	skillsRegistry := skills.NewRegistry()

	// Register skill tools (always available, content varies by features)
	registry.Register(&tools.ListSkillsTool{BaseTool: base, Registry: skillsRegistry})
	registry.Register(&tools.RunSkillTool{BaseTool: base, Registry: skillsRegistry})

	// Create MCP server
	srv := mcpserver.NewServer(registry)

	// Register remediation tool (always available)
	registry.Register(&tools.SuggestRemediationTool{BaseTool: base})

	// Gateway API tool names for conditional registration
	gatewayToolNames := []string{"list_gateways", "get_gateway", "list_httproutes", "get_httproute", "list_grpcroutes", "get_grpcroute", "list_referencegrants", "get_referencegrant", "scan_gateway_misconfigs", "check_gateway_conformance", "design_gateway_api"}
	istioToolNames := []string{"list_istio_resources", "get_istio_resource", "check_sidecar_injection", "check_istio_mtls", "validate_istio_config", "analyze_istio_authpolicy", "analyze_istio_routing", "design_istio"}

	kgatewayToolNames := []string{"list_kgateway_resources", "validate_kgateway_resource", "check_kgateway_health", "design_kgateway"}
	kumaToolNames := []string{"check_kuma_status"}
	linkerdToolNames := []string{"check_linkerd_status"}
	ciliumToolNames := []string{"list_cilium_policies", "check_cilium_status"}
	calicoToolNames := []string{"list_calico_policies", "check_calico_status"}
	flannelToolNames := []string{"check_flannel_status"}

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
			registry.Register(&tools.DesignGatewayAPITool{BaseTool: base})
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
			registry.Register(&tools.AnalyzeIstioAuthPolicyTool{BaseTool: base})
			registry.Register(&tools.AnalyzeIstioRoutingTool{BaseTool: base})
			registry.Register(&tools.DesignIstioTool{BaseTool: base})
		} else {
			for _, name := range istioToolNames {
				registry.Unregister(name)
			}
		}

		// kgateway tools
		if features.HasKgateway {
			registry.Register(&tools.ListKgatewayResourcesTool{BaseTool: base})
			registry.Register(&tools.ValidateKgatewayResourceTool{BaseTool: base})
			registry.Register(&tools.CheckKgatewayHealthTool{BaseTool: base})
			registry.Register(&tools.DesignKgatewayTool{BaseTool: base})
		} else {
			for _, name := range kgatewayToolNames {
				registry.Unregister(name)
			}
		}

		// Kuma tools
		if features.HasKuma {
			registry.Register(&tools.CheckKumaStatusTool{BaseTool: base})
		} else {
			for _, name := range kumaToolNames {
				registry.Unregister(name)
			}
		}

		// Linkerd tools
		if features.HasLinkerd {
			registry.Register(&tools.CheckLinkerdStatusTool{BaseTool: base})
		} else {
			for _, name := range linkerdToolNames {
				registry.Unregister(name)
			}
		}

		// Cilium tools
		if features.HasCilium {
			registry.Register(&tools.ListCiliumPoliciesTool{BaseTool: base})
			registry.Register(&tools.CheckCiliumStatusTool{BaseTool: base})
		} else {
			for _, name := range ciliumToolNames {
				registry.Unregister(name)
			}
		}

		// Calico tools
		if features.HasCalico {
			registry.Register(&tools.ListCalicoPoliciesTool{BaseTool: base})
			registry.Register(&tools.CheckCalicoStatusTool{BaseTool: base})
		} else {
			for _, name := range calicoToolNames {
				registry.Unregister(name)
			}
		}

		// Flannel tools
		if features.HasFlannel {
			registry.Register(&tools.CheckFlannelStatusTool{BaseTool: base})
		} else {
			for _, name := range flannelToolNames {
				registry.Unregister(name)
			}
		}

		// Sync skills registry with discovered features
		skillsRegistry.SyncWithFeatures(features, cfg, clients)

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
		_, _ = fmt.Fprint(w, "ok")
	})
	healthMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if !disc.IsReady() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprint(w, "not ready: initial CRD discovery pending")
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "ok")
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

	probeMgr.Stop()

	// Flush pending OTel data (traces + metrics + logs) before exit
	if err := otelResult.Shutdown(shutdownCtx); err != nil {
		slog.Error("telemetry shutdown error", "error", err)
	}

	slog.Info("server stopped")
}
