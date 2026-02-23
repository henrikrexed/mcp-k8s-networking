package discovery

import (
	"log"
	"sync"
	"time"

	"k8s.io/client-go/discovery"
)

type Features struct {
	HasGatewayAPI bool
	HasIstio      bool
	HasCilium     bool
	HasCalico     bool
	HasLinkerd    bool
}

type OnChangeFunc func(Features)

type Discovery struct {
	client   discovery.DiscoveryInterface
	features Features
	onChange OnChangeFunc
	mu       sync.RWMutex
	stopCh   chan struct{}
}

func New(client discovery.DiscoveryInterface, onChange OnChangeFunc) *Discovery {
	return &Discovery{
		client:   client,
		onChange: onChange,
		stopCh:   make(chan struct{}),
	}
}

func (d *Discovery) GetFeatures() Features {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.features
}

func (d *Discovery) Start() {
	d.poll()
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				d.poll()
			case <-d.stopCh:
				return
			}
		}
	}()
}

func (d *Discovery) Stop() {
	close(d.stopCh)
}

func (d *Discovery) poll() {
	groups, err := d.client.ServerGroups()
	if err != nil {
		log.Printf("discovery: failed to fetch server groups: %v", err)
		return
	}

	newFeatures := Features{}
	for _, group := range groups.Groups {
		switch group.Name {
		case "gateway.networking.k8s.io":
			newFeatures.HasGatewayAPI = true
		case "networking.istio.io", "security.istio.io":
			newFeatures.HasIstio = true
		case "cilium.io":
			newFeatures.HasCilium = true
		case "crd.projectcalico.org":
			newFeatures.HasCalico = true
		case "linkerd.io":
			newFeatures.HasLinkerd = true
		}
	}

	d.mu.Lock()
	changed := newFeatures != d.features
	d.features = newFeatures
	d.mu.Unlock()

	if changed && d.onChange != nil {
		d.onChange(newFeatures)
	}
}
