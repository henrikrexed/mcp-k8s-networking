package skills

import (
	"context"
	"sync"

	"github.com/isitobservable/k8s-networking-mcp/pkg/config"
	"github.com/isitobservable/k8s-networking-mcp/pkg/discovery"
	"github.com/isitobservable/k8s-networking-mcp/pkg/k8s"
)

// Skill is the interface all skill implementations must satisfy.
type Skill interface {
	Definition() SkillDefinition
	Execute(ctx context.Context, args map[string]interface{}) (*SkillResult, error)
}

// Registry manages available skills based on CRD availability.
type Registry struct {
	skills map[string]Skill
	mu     sync.RWMutex
}

// NewRegistry creates a skill registry.
func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string]Skill),
	}
}

// Register adds a skill to the registry.
func (r *Registry) Register(s Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[s.Definition().Name] = s
}

// Unregister removes a skill from the registry.
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.skills, name)
}

// Get retrieves a skill by name.
func (r *Registry) Get(name string) (Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[name]
	return s, ok
}

// List returns all registered skill definitions.
func (r *Registry) List() []SkillDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]SkillDefinition, 0, len(r.skills))
	for _, s := range r.skills {
		defs = append(defs, s.Definition())
	}
	return defs
}

// SyncWithFeatures registers/unregisters skills based on discovered features.
func (r *Registry) SyncWithFeatures(features discovery.Features, cfg *config.Config, clients *k8s.Clients) {
	base := skillBase{cfg: cfg, clients: clients}

	// Gateway API skills
	if features.HasGatewayAPI {
		r.Register(&ExposeServiceSkill{base: base})
	} else {
		r.Unregister("expose_service_gateway_api")
	}

	// Istio skills
	if features.HasIstio {
		r.Register(&ConfigureMTLSSkill{base: base})
	} else {
		r.Unregister("configure_istio_mtls")
	}

	// Traffic split (needs Istio or Gateway API)
	if features.HasIstio || features.HasGatewayAPI {
		r.Register(&TrafficSplitSkill{base: base, hasIstio: features.HasIstio, hasGatewayAPI: features.HasGatewayAPI})
	} else {
		r.Unregister("configure_traffic_split")
	}

	// NetworkPolicy (always available)
	r.Register(&NetworkPolicySkill{base: base, hasCilium: features.HasCilium, hasCalico: features.HasCalico})
}

// skillBase provides shared dependencies for skill implementations.
type skillBase struct {
	cfg     *config.Config
	clients *k8s.Clients
}
