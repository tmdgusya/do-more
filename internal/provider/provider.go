package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type Provider interface {
	Name() string
	Run(ctx context.Context, prompt string, workDir string) (string, error)
}

type ProviderRegistry struct {
	providers map[string]Provider
}

func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]Provider),
	}
}

func (r *ProviderRegistry) Register(p Provider) {
	r.providers[p.Name()] = p
}

func (r *ProviderRegistry) Get(name string) (Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

func (r *ProviderRegistry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func FormatModels(available []string, configured string) string {
	var b strings.Builder
	for _, name := range available {
		if name == configured {
			fmt.Fprintf(&b, "  * %s (configured)\n", name)
		} else {
			fmt.Fprintf(&b, "  - %s\n", name)
		}
	}
	return b.String()
}
