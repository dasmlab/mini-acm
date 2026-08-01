package mockup

import "fmt"

// Genre / Style identify which product offering a MockUp belongs to.
// Resource kind stays "MockUp"; genre is the product family, style is the template.
const (
	GenreClusterManagement       = "cluster-management"
	GenreApplicationDevelopment  = "application-development"
	GenreInfrastructure          = "infrastructure"
	GenreContentDelivery         = "content-delivery"

	StyleACMMultiCluster = "acm-multi-cluster"
	StyleSingleSNOOCP        = "single-sno-ocp"
	StyleWindowsUI           = "windows-ui"
	StyleWebFullStack        = "web-full-stack"
	StyleInfraNodeNetwork    = "infra-node-network-payload"
	StyleSurfingCdnR2        = "surfing-cdn-r2"
)

// RelationRule constrains how object types may connect (validate / palette later).
type RelationRule struct {
	From        string `json:"from"`
	Rel         string `json:"rel"`
	To          string `json:"to"`
	Cardinality string `json:"cardinality,omitempty"` // e.g. "1..1", "1..*", "0..*"
	Notes       string `json:"notes,omitempty"`
}

// StyleDef is a creatable (or stub) template within a genre.
type StyleDef struct {
	ID           string         `json:"id"`
	Genre        string         `json:"genre"`
	Label        string         `json:"label"`
	Description  string         `json:"description"`
	Available    bool           `json:"available"` // false = catalog stub, create rejected
	ObjectTypes  []string       `json:"objectTypes"`
	Views        []string       `json:"views,omitempty"`
	Relations    []RelationRule `json:"relations,omitempty"`
	DefaultSeed  string         `json:"defaultSeed,omitempty"` // human hint
}

// GenreDef groups styles.
type GenreDef struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Styles      []string `json:"styles"` // style ids
}

// CatalogResponse is GET /api/v1/catalog.
type CatalogResponse struct {
	Genres []GenreDef `json:"genres"`
	Styles []StyleDef `json:"styles"`
}

// Catalog returns the product genre/style registry (code-defined for now;
// later: data/genres/*.yaml + "Add a Genre" UI).
func Catalog() CatalogResponse {
	styles := []StyleDef{
		{
			ID: StyleSingleSNOOCP, Genre: GenreClusterManagement,
			Label: "Single SNO OCP",
			Description: "Bring up one SNO (OCP-MGMT) via the adapter (libvirt today). Same MACHINE-HOST → vHost path as the mgmt half of ACM Multi-Cluster — stop before ACM / spokes.",
			Available: true,
			ObjectTypes: []string{
				"MachineHost", "Adapter", "VHost", "Gateway", "OCP-MGMT", "Appliance",
			},
			Views:       []string{"all", "infra", "network", "cluster"},
			DefaultSeed: "lab rack with MACHINE-HOST + VyOS + one SNO OCP-MGMT (no ACM, no deployments)",
			Relations: []RelationRule{
				{From: "Adapter", Rel: "runsOn", To: "MachineHost", Cardinality: "1..1"},
				{From: "VHost", Rel: "hostedBy", To: "Adapter", Cardinality: "1..*"},
				{From: "Gateway", Rel: "runsOn", To: "VHost", Cardinality: "1..1", Notes: "VyOS on vHost-GW"},
				{From: "OCP-MGMT", Rel: "runsOn", To: "VHost", Cardinality: "1..1", Notes: "SNO guest — same Hub object used by ACM Multi-Cluster"},
			},
		},
		{
			ID: StyleACMMultiCluster, Genre: GenreClusterManagement,
			Label: "ACM Multi-Cluster",
			Description: "Composable: Single SNO (OCP-MGMT) + ACM payload + N× OCP-DEPLOY. Mgmt hosts ACM; managed deployments on the lab rack.",
			Available: true,
			ObjectTypes: []string{
				"MachineHost", "Adapter", "VHost", "Gateway", "OCP-MGMT", "ACM", "OCP-DEPLOY", "Appliance",
			},
			Views: []string{"all", "infra", "network", "cluster", "app"},
			DefaultSeed: "lab rack with OCP-MGMT + ACM + 2 OCP-DEPLOY clusters",
			Relations: []RelationRule{
				{From: "Adapter", Rel: "runsOn", To: "MachineHost", Cardinality: "1..1"},
				{From: "VHost", Rel: "hostedBy", To: "Adapter", Cardinality: "1..*"},
				{From: "Gateway", Rel: "runsOn", To: "VHost", Cardinality: "1..1", Notes: "VyOS on vHost-GW"},
				{From: "OCP-MGMT", Rel: "runsOn", To: "VHost", Cardinality: "1..1", Notes: "MGMT SNO guest"},
				{From: "OCP-DEPLOY", Rel: "runsOn", To: "VHost", Cardinality: "1..*", Notes: "cp/worker guests"},
				{From: "ACM", Rel: "runsOn", To: "OCP-MGMT", Cardinality: "1..1"},
				{From: "OCP-DEPLOY", Rel: "managedBy", To: "ACM", Cardinality: "1..*"},
			},
		},
		{
			ID: StyleWindowsUI, Genre: GenreApplicationDevelopment,
			Label: "Windows UI MockUp",
			Description: "Client app SDLC canvas: OS → runtime (.NET/WPF, …) → UI surfaces → data/devices/services. Inspired by apps like running-translate.",
			Available: false,
			ObjectTypes: []string{
				"RunningOS", "ClientRuntime", "Window", "Form", "Control", "DataInput", "Device", "DataOutput", "ServiceCall",
			},
			Views: []string{"all", "runtime", "ui", "dataflow"},
			DefaultSeed: "stub — empty Windows UI canvas (not seeded yet)",
			Relations: []RelationRule{
				{From: "ClientRuntime", Rel: "runsOn", To: "RunningOS", Cardinality: "1..1"},
				{From: "Window", Rel: "hostedBy", To: "ClientRuntime", Cardinality: "1..*"},
				{From: "Form", Rel: "contains", To: "Control", Cardinality: "0..*"},
				{From: "Form", Rel: "navigatesTo", To: "Form", Cardinality: "0..*"},
				{From: "Form", Rel: "reads", To: "DataInput", Cardinality: "0..*"},
				{From: "Form", Rel: "writes", To: "DataOutput", Cardinality: "0..*"},
				{From: "Form", Rel: "calls", To: "ServiceCall", Cardinality: "0..*"},
			},
		},
		{
			ID: StyleWebFullStack, Genre: GenreApplicationDevelopment,
			Label: "Web Full-Stack Application",
			Description: "Routes, FE/BE components, APIs, data stores — bread-and-butter app MockUps.",
			Available: false,
			ObjectTypes: []string{
				"Route", "Frontend", "Backend", "API", "DataStore", "Auth", "ExternalService",
			},
			Views: []string{"all", "frontend", "backend", "data"},
			DefaultSeed: "stub — empty web stack canvas (not seeded yet)",
			Relations: []RelationRule{
				{From: "Frontend", Rel: "calls", To: "API", Cardinality: "0..*"},
				{From: "Backend", Rel: "exposes", To: "API", Cardinality: "0..*"},
				{From: "Backend", Rel: "uses", To: "DataStore", Cardinality: "0..*"},
				{From: "Route", Rel: "renders", To: "Frontend", Cardinality: "1..*"},
			},
		},
		{
			ID: StyleInfraNodeNetwork, Genre: GenreInfrastructure,
			Label: "Infra · Node · Network · Payload",
			Description: "Standalone infrastructure MockUp (hosts, nets, payloads) without ACM governance.",
			Available: false,
			ObjectTypes: []string{"MachineHost", "Adapter", "VHost", "Network", "Appliance", "Payload"},
			Views:       []string{"all", "infra", "network", "payload"},
			DefaultSeed: "stub — infra-focused canvas (not seeded yet)",
			Relations: []RelationRule{
				{From: "Adapter", Rel: "runsOn", To: "MachineHost", Cardinality: "1..1"},
				{From: "VHost", Rel: "hostedBy", To: "Adapter", Cardinality: "1..*"},
				{From: "Payload", Rel: "runsOn", To: "VHost", Cardinality: "0..*"},
			},
		},
		{
			ID: StyleSurfingCdnR2, Genre: GenreContentDelivery,
			Label: "Surfing CDN · R2 + Cloudflare",
			Description: "Mock a CDN network: DC Bound origin house (thin up / Surfing API) → Cloudflare edge → optional R2 object store + DAM. Profile cost to cheapcloud ($20/mo storage+CDN envelope). Production twin: dasmlab_home surfing-service.",
			Available: false,
			ObjectTypes: []string{
				"SourceHouse", "OriginApp", "EdgeCDN", "ObjectStore", "DAM", "MediaRoute", "CostProfile",
			},
			Views:       []string{"all", "origin", "edge", "storage", "cost"},
			DefaultSeed: "stub — Bound origin → CF CDN → optional R2 (not seeded yet)",
			Relations: []RelationRule{
				{From: "OriginApp", Rel: "runsOn", To: "SourceHouse", Cardinality: "1..1", Notes: "OCP Surfing / thin-up in Bound DC"},
				{From: "OriginApp", Rel: "publishesTo", To: "ObjectStore", Cardinality: "0..1", Notes: "optional cloud origin for bytes"},
				{From: "EdgeCDN", Rel: "pullsFrom", To: "ObjectStore", Cardinality: "0..1"},
				{From: "EdgeCDN", Rel: "pullsFrom", To: "OriginApp", Cardinality: "0..1", Notes: "pull-through when no object store"},
				{From: "MediaRoute", Rel: "terminatesAt", To: "EdgeCDN", Cardinality: "1..1"},
				{From: "DAM", Rel: "indexes", To: "ObjectStore", Cardinality: "0..1"},
				{From: "CostProfile", Rel: "constrains", To: "ObjectStore", Cardinality: "0..1", Notes: "cheapcloud MediaBroker envelope"},
				{From: "CostProfile", Rel: "constrains", To: "EdgeCDN", Cardinality: "0..1"},
			},
		},
	}

	genres := []GenreDef{
		{
			ID: GenreClusterManagement, Label: "Cluster Management",
			Description: "OCP lab MockUps via adapter (libvirt…): Single SNO, or mock-me (SNO + ACM + managed deployments). Building blocks: vHost, OCP-MGMT, OCP-DEPLOY, VyOS, HAP, ACM.",
			Styles: []string{StyleSingleSNOOCP, StyleACMMultiCluster},
		},
		{
			ID: GenreApplicationDevelopment, Label: "Application Development",
			Description: "Client and full-stack application design MockUps (UI, runtime, dataflow).",
			Styles: []string{StyleWindowsUI, StyleWebFullStack},
		},
		{
			ID: GenreInfrastructure, Label: "Infrastructure",
			Description: "Hosts, networks, and payloads without a cluster-management control plane.",
			Styles: []string{StyleInfraNodeNetwork},
		},
		{
			ID: GenreContentDelivery, Label: "Content Delivery",
			Description: "CDN / origin / object-store MockUps (Surfing pattern). Profile into cheapcloud for cheapest live path under budget.",
			Styles: []string{StyleSurfingCdnR2},
		},
	}

	return CatalogResponse{Genres: genres, Styles: styles}
}

// LookupStyle returns a style definition or nil.
func LookupStyle(id string) *StyleDef {
	for _, s := range Catalog().Styles {
		if s.ID == id {
			cp := s
			return &cp
		}
	}
	return nil
}

// ResolveCreateStyle picks genre/style for Create, defaulting to mock-me.
func ResolveCreateStyle(genre, style string) (g, st string, def *StyleDef, err error) {
	if style == "" {
		style = StyleACMMultiCluster
	}
	def = LookupStyle(style)
	if def == nil {
		return "", "", nil, fmt.Errorf("unknown style %q", style)
	}
	if genre == "" {
		genre = def.Genre
	}
	if genre != def.Genre {
		return "", "", nil, fmt.Errorf("style %q belongs to genre %q, not %q", style, def.Genre, genre)
	}
	if !def.Available {
		return "", "", nil, fmt.Errorf("style %q (%s) is not creatable yet — catalog stub", style, def.Label)
	}
	return genre, style, def, nil
}
