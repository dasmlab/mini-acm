package main

import (
	"github.com/spf13/cobra"

	// Register providers.
	_ "github.com/dasmlab/mini-acm/internal/provider/libvirt"
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
		Use:   "mini-acm",
		Short: "Lab orchestrator for ACM hub SNO + compact managed clusters",
		Long: banner + `
Typical flow:

  1. cp .env.example .env   # PULL_SECRET_FILE, SSH key
  2. cp config/hub.example.yaml hub.yaml
  3. mini-acm hub create --config hub.yaml --manual
  4. # after hub is up + ACM installed
  5. cp config/cluster.example.yaml cluster.yaml
  6. mini-acm cluster create --config cluster.yaml --manual

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

	return root
}
