// Command mock-me orchestrates small lab OpenShift hubs (SNO + ACM) and
// ACM-managed compact clusters on pluggable infrastructure providers.
//
// LAB / TEST / DEV ONLY. Not a supported production installer.
package main

import (
	"fmt"
	"os"
)

const banner = `
================================================================================
  mock-me — LAB / TEST / DEV ONLY

  Builds a virtual rack for ACM lifecycle demos:
    hub create     → local Agent-based SNO + optional ACM
    cluster create → 3 libvirt VMs + ACM Agent/InfraEnv path

  Secrets stay in env / files (.env). Never commit pull secrets.
================================================================================
`

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
