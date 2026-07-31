// Package provider defines the infrastructure adapter surface.
// ACM owns OpenShift lifecycle; providers only present bootable machines.
package provider

import (
	"context"
	"fmt"
)

// PowerState is a coarse node power status.
type PowerState string

const (
	PowerUnknown PowerState = "unknown"
	PowerOn      PowerState = "on"
	PowerOff     PowerState = "off"
)

// NodeSpec describes a VM (or bare-metal stand-in) to create.
type NodeSpec struct {
	Name      string
	CPU       int
	MemoryMiB int
	DiskGiB   int
	MAC       string
	IP        string // informational / DHCP reservation hint
	Network   string
	Pool      string
	ISOPath   string // optional at create time
}

// NetworkSpec describes an isolated lab network.
type NetworkSpec struct {
	Name      string
	Bridge    string
	CIDR      string
	Gateway   string
	DHCPStart string
	DHCPEnd   string
	Forward   string // nat | none | route
}

// Provider is the infra adapter contract.
type Provider interface {
	Name() string
	EnsureNetwork(ctx context.Context, net NetworkSpec) error
	CreateNode(ctx context.Context, node NodeSpec) error
	DeleteNode(ctx context.Context, name string) error
	AttachISO(ctx context.Context, name, isoPath string) error
	DetachISO(ctx context.Context, name string) error
	StartNode(ctx context.Context, name string) error
	StopNode(ctx context.Context, name string) error
	GetMAC(ctx context.Context, name string) (string, error)
	GetPowerState(ctx context.Context, name string) (PowerState, error)
	ListNodes(ctx context.Context, prefix string) ([]string, error)
}

// Options control dry-run / manual behavior for all providers.
type Options struct {
	DryRun bool
	Manual bool // print commands instead of executing destructive virt ops
}

// Registry maps provider type → factory.
type Factory func(opts Options) (Provider, error)

var factories = map[string]Factory{}

// Register adds a provider factory (called from init in adapter packages).
func Register(typeName string, f Factory) {
	factories[typeName] = f
}

// New returns a provider by type name.
func New(typeName string, opts Options) (Provider, error) {
	f, ok := factories[typeName]
	if !ok {
		return nil, fmt.Errorf("unknown provider type %q (registered: %v)", typeName, registeredNames())
	}
	return f(opts)
}

func registeredNames() []string {
	out := make([]string, 0, len(factories))
	for k := range factories {
		out = append(out, k)
	}
	return out
}
