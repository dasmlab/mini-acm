package cheapcloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dasmlab/mock-me/internal/mockup"
)

// DefaultURL is the prod-1 OCP Route (no HAProxy basic auth).
const DefaultURL = "https://cheapcloud-dasmlab.apps.2026-prod-1.ocp.dasmlab.org"

// Client talks to cheapcloud COST-ME.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewFromEnv() *Client {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("CHEAPCLOUD_URL")), "/")
	if base == "" {
		base = DefaultURL
	}
	return &Client{
		BaseURL: base,
		HTTP:    &http.Client{Timeout: 45 * time.Second},
	}
}

// Target mirrors cheapcloud CostMeTarget.
type Target struct {
	Capability   string  `json:"capability"`
	Provider     string  `json:"provider,omitempty"`
	Count        int     `json:"count,omitempty"`
	Spot         *bool   `json:"spot,omitempty"`
	RegionHint   string  `json:"region_hint,omitempty"`
	SKUHint      string  `json:"sku_hint,omitempty"`
	StorageGBEst float64 `json:"storage_gb_est,omitempty"`
}

// Request is POST /api/v1/cost-me body.
type Request struct {
	ProductID         string   `json:"product_id,omitempty"`
	MockupID          string   `json:"mockup_id,omitempty"`
	RegisterFootprint bool     `json:"register_footprint,omitempty"`
	Targets           []Target `json:"targets"`
}

// Report is the cheapcloud response (opaque map + typed convenience fields).
type Report map[string]any

// CostMe calls cheapcloud and returns the report JSON.
func (c *Client) CostMe(req Request) (Report, error) {
	if c == nil || c.BaseURL == "" {
		return nil, fmt.Errorf("cheapcloud client not configured")
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	url := c.BaseURL + "/api/v1/cost-me"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	res, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("cheapcloud cost-me: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("cheapcloud cost-me %s: %s", res.Status, truncate(string(raw), 300))
	}
	var report Report
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, fmt.Errorf("decode cost-me: %w", err)
	}
	return report, nil
}

// TargetsFromMockUp maps style → COST-ME capability targets.
func TargetsFromMockUp(m *mockup.MockUp) []Target {
	if m == nil {
		return nil
	}
	spot := true
	style := m.Spec.Style
	switch style {
	case mockup.StyleSingleSNOOCP:
		return []Target{{
			Capability: "ocp-sno-slim", Provider: "azure", Count: 1, Spot: &spot,
		}}
	case mockup.StyleACMMultiCluster, "":
		n := 1 + len(m.Spec.Clusters) // hub SNO + deployments
		if n < 1 {
			n = 1
		}
		return []Target{{
			Capability: "ocp-sno-slim", Provider: "azure", Count: n, Spot: &spot,
		}}
	case mockup.StyleSurfingCdnR2, mockup.StyleSelfServePersonalCDN:
		return []Target{{
			Capability: "object-store", Provider: "r2", StorageGBEst: 8,
		}}
	default:
		return []Target{{
			Capability: "ocp-sno-slim", Provider: "azure", Count: 1, Spot: &spot,
		}}
	}
}

func ProductID(m *mockup.MockUp) string {
	if m == nil {
		return "mock-me"
	}
	name := strings.TrimSpace(m.Metadata.Name)
	if name == "" {
		name = m.Metadata.ID
	}
	return "mock-me-" + name
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
