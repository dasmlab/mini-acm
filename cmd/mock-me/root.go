package main

import (
	"github.com/spf13/cobra"

	// Register providers.
	_ "github.com/dasmlab/mock-me/internal/provider/libvirt"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

type globalFlags struct {
	kubeconfig string
	dryRun     bool
	manual     bool
}

func newRootCmd() *cobra.Command {
	gf := &globalFlags{}

	root := &cobra.Command{
		Use:   "mock-me",
		Short: "Lab orchestrator for ACM hub SNO + compact managed clusters",
		Long: banner + `
Typical flow:

  1. mock-me serve                     # UI: MockUp → Topology → Wizard
  2. Derive YAML from MockUp, or:
  3. cp .env.example .env + hub/cluster YAML
  4. mock-me hub create --config hub.yaml --manual
  5. mock-me hub install-acm
  6. mock-me cluster create --manual
  7. mock-me cluster attach-iso --iso discovery.iso

Provider layer presents bootable machines. ACM owns OCP install lifecycle.

Auth for hub ACM ops:
  --kubeconfig | $KUBECONFIG | hub workdir auth/kubeconfig
`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&gf.kubeconfig, "kubeconfig", "", "hub kubeconfig for ACM operations")
	root.PersistentFlags().BoolVar(&gf.dryRun, "dry-run", false, "print actions without executing")
	root.PersistentFlags().BoolVar(&gf.manual, "manual", false, "print provider/install commands for hand-run steps")

	root.AddCommand(newHubCmd(gf))
	root.AddCommand(newClusterCmd(gf))
	root.AddCommand(newServeCmd())
	root.AddCommand(newLabE2ECmd())

	return root
}
