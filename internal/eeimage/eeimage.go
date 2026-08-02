// Package eeimage names the curated mock-me execution environment image.
// Inventory hosts only need podman; openshift-install + oc live in this image.
package eeimage

import (
	"os"
	"strings"
)

// Default is published to GHCR for lab Deploy (OCP 4.18 client tools).
const Default = "ghcr.io/dasmlab/mock-me-ee:4.18"

// Image returns MOCK_ME_EE_IMAGE or Default.
func Image() string {
	if v := strings.TrimSpace(os.Getenv("MOCK_ME_EE_IMAGE")); v != "" {
		return v
	}
	return Default
}
