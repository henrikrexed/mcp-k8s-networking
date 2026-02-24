package discovery

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
)

type Features struct {
	HasGatewayAPI bool
	HasIstio      bool
	HasCilium     bool
	HasCalico     bool
	HasLinkerd    bool
	HasKuma       bool
	HasFlannel    bool
	HasKgateway   bool
}

type ProviderInfo struct {
	Name     string `json:"name"`
	APIGroup string `json:"apiGroup"`
	Version  string `json:"version"`
	Detected bool   `json:"detected"`
}

type OnChangeFunc func(Features)

type Discovery struct {
	discoveryClient discovery.DiscoveryInterface
	dynamicClient   dynamic.Interface
	features        Features
	onChange        OnChangeFunc
	mu              sync.RWMutex
	cancel          context.CancelFunc
	ready           bool

	providerVersions map[string]string
}

func New(discoveryClient discovery.DiscoveryInterface, dynamicClient dynamic.Interface, onChange OnChangeFunc) *Discovery {
	return &Discovery{
		discoveryClient:  discoveryClient,
		dynamicClient:    dynamicClient,
		onChange:         onChange,
		providerVersions: make(map[string]string),
	}
}

func (d *Discovery) GetFeatures() Features {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.features
}

// IsReady returns true after the initial CRD scan has completed.
func (d *Discovery) IsReady() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.ready
}

// GetProviders returns information about all detected networking providers.
func (d *Discovery) GetProviders() []ProviderInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()

	providers := []ProviderInfo{
		{Name: "Gateway API", APIGroup: "gateway.networking.k8s.io", Detected: d.features.HasGatewayAPI},
		{Name: "Istio", APIGroup: "networking.istio.io", Detected: d.features.HasIstio},
		{Name: "Cilium", APIGroup: "cilium.io", Detected: d.features.HasCilium},
		{Name: "Calico", APIGroup: "crd.projectcalico.org", Detected: d.features.HasCalico},
		{Name: "Linkerd", APIGroup: "linkerd.io", Detected: d.features.HasLinkerd},
		{Name: "Kuma", APIGroup: "kuma.io", Detected: d.features.HasKuma},
		{Name: "Flannel", APIGroup: "", Detected: d.features.HasFlannel},
		{Name: "kgateway", APIGroup: "kgateway.dev", Detected: d.features.HasKgateway},
	}

	for i := range providers {
		if v, ok := d.providerVersions[providers[i].APIGroup]; ok {
			providers[i].Version = v
		}
	}

	return providers
}

// Start performs initial CRD scan and then starts watching for CRD changes.
func (d *Discovery) Start(ctx context.Context) {
	ctx, d.cancel = context.WithCancel(ctx)

	// Initial scan via ServerGroups (fast)
	d.initialScan()

	d.mu.Lock()
	d.ready = true
	d.mu.Unlock()
	slog.Info("discovery: initial scan complete, ready")

	// Start CRD watch in background
	go d.watchLoop(ctx)
}

func (d *Discovery) Stop() {
	if d.cancel != nil {
		d.cancel()
	}
}

// initialScan uses the discovery client for fast initial detection.
func (d *Discovery) initialScan() {
	groups, err := d.discoveryClient.ServerGroups()
	if err != nil {
		slog.Error("discovery: failed to fetch server groups", "error", err)
		return
	}

	newFeatures := Features{}
	versions := make(map[string]string)

	for _, group := range groups.Groups {
		d.detectGroup(group.Name, group.PreferredVersion.Version, &newFeatures, versions)
	}

	d.mu.Lock()
	changed := newFeatures != d.features
	d.features = newFeatures
	d.providerVersions = versions
	d.mu.Unlock()

	if changed && d.onChange != nil {
		d.onChange(newFeatures)
	}
}

var crdGVR = schema.GroupVersionResource{
	Group:    "apiextensions.k8s.io",
	Version:  "v1",
	Resource: "customresourcedefinitions",
}

// watchLoop watches CRD resources and reconnects with exponential backoff on errors.
func (d *Discovery) watchLoop(ctx context.Context) {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		watcher, err := d.dynamicClient.Resource(crdGVR).Watch(ctx, metav1.ListOptions{})
		if err != nil {
			slog.Error("discovery: failed to start CRD watch", "error", err, "retryIn", backoff)
			select {
			case <-time.After(backoff):
				backoff = min(backoff*2, maxBackoff)
				continue
			case <-ctx.Done():
				return
			}
		}

		// Reset backoff on successful connection
		backoff = time.Second
		slog.Info("discovery: CRD watch established")

		d.processEvents(ctx, watcher)
		watcher.Stop()
	}
}

func (d *Discovery) processEvents(ctx context.Context, watcher watch.Interface) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				slog.Warn("discovery: CRD watch channel closed, reconnecting")
				return
			}

			if event.Type != watch.Added && event.Type != watch.Deleted {
				continue
			}

			obj, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}

			group, _, _ := unstructured.NestedString(obj.Object, "spec", "group")
			if group == "" {
				continue
			}

			slog.Debug("discovery: CRD event", "type", event.Type, "group", group)

			// Rescan all CRDs to recompute features
			d.rescanCRDs(ctx)
		}
	}
}

// rescanCRDs lists all CRDs and recomputes the features set.
func (d *Discovery) rescanCRDs(ctx context.Context) {
	crdList, err := d.dynamicClient.Resource(crdGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Error("discovery: failed to list CRDs", "error", err)
		return
	}

	newFeatures := Features{}
	versions := make(map[string]string)

	for _, item := range crdList.Items {
		group, _, _ := unstructured.NestedString(item.Object, "spec", "group")
		version := extractPreferredVersion(&item)
		if group != "" {
			d.detectGroup(group, version, &newFeatures, versions)
		}
	}

	d.mu.Lock()
	changed := newFeatures != d.features
	d.features = newFeatures
	d.providerVersions = versions
	d.mu.Unlock()

	if changed && d.onChange != nil {
		slog.Info("discovery: features changed",
			"gatewayAPI", newFeatures.HasGatewayAPI,
			"istio", newFeatures.HasIstio,
			"cilium", newFeatures.HasCilium,
			"calico", newFeatures.HasCalico,
			"linkerd", newFeatures.HasLinkerd,
			"kuma", newFeatures.HasKuma,
			"flannel", newFeatures.HasFlannel,
			"kgateway", newFeatures.HasKgateway,
		)
		d.onChange(newFeatures)
	}
}

// detectGroup maps a CRD API group to the corresponding feature flag.
func (d *Discovery) detectGroup(group, version string, features *Features, versions map[string]string) {
	switch {
	case group == "gateway.networking.k8s.io":
		features.HasGatewayAPI = true
		versions[group] = version
	case group == "networking.istio.io" || group == "security.istio.io":
		features.HasIstio = true
		versions["networking.istio.io"] = version
	case group == "cilium.io":
		features.HasCilium = true
		versions[group] = version
	case group == "crd.projectcalico.org":
		features.HasCalico = true
		versions[group] = version
	case group == "linkerd.io":
		features.HasLinkerd = true
		versions[group] = version
	case group == "kuma.io":
		features.HasKuma = true
		versions[group] = version
	case group == "kgateway.dev" || strings.HasSuffix(group, ".kgateway.dev"):
		features.HasKgateway = true
		versions["kgateway.dev"] = version
	}
}

// extractPreferredVersion gets the preferred served version from a CRD object.
func extractPreferredVersion(crd *unstructured.Unstructured) string {
	versions, found, err := unstructured.NestedSlice(crd.Object, "spec", "versions")
	if err != nil || !found {
		return ""
	}
	for _, v := range versions {
		if vm, ok := v.(map[string]interface{}); ok {
			if served, _, _ := unstructured.NestedBool(vm, "served"); served {
				if name, _, _ := unstructured.NestedString(vm, "name"); name != "" {
					return name
				}
			}
		}
	}
	return ""
}
