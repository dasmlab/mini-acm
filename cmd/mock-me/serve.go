package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"github.com/dasmlab/mock-me/internal/activity"
	"github.com/dasmlab/mock-me/internal/api"
	"github.com/dasmlab/mock-me/internal/auth"
	"github.com/dasmlab/mock-me/internal/inventory"
	"github.com/dasmlab/mock-me/internal/mockup"
)

//go:embed all:static
var staticEmbed embed.FS

func newServeCmd() *cobra.Command {
	var (
		dataDir string
		addr    string
	)
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the web UI + MockUp API",
		Long: `serve embeds the Vue UI and exposes /api/v1 for MockUp topology + wizard flows.

  mock-me serve --listen :8080 --data-dir ./data

LAB / TEST / DEV ONLY.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			if dataDir == "" {
				dataDir = os.Getenv("DATA_DIR")
			}
			if dataDir == "" {
				dataDir = "./data"
			}
			if addr == "" {
				addr = os.Getenv("LISTEN_ADDR")
			}
			if addr == "" {
				addr = ":8080"
			}
			if err := os.MkdirAll(dataDir, 0o755); err != nil {
				return err
			}
			store, err := mockup.NewStore(dataDir)
			if err != nil {
				return err
			}
			inv, err := inventory.NewStore(dataDir)
			if err != nil {
				return err
			}
			act, err := activity.NewStore(dataDir)
			if err != nil {
				return err
			}

			authCfg := auth.ConfigFromEnv()
			authSvc, err := auth.New(context.Background(), authCfg)
			if err != nil {
				return fmt.Errorf("oidc: %w", err)
			}
			if authSvc.Enabled() {
				fmt.Fprintf(os.Stderr, "OIDC enabled (issuer=%s client=%s)\n", authCfg.Issuer, authCfg.ClientID)
			} else {
				fmt.Fprintln(os.Stderr, "OIDC disabled — open local/dev mode (set KEYCLOAK_URL + OIDC_CLIENT_SECRET to enable)")
			}

			var staticHandler http.Handler
			sub, err := fs.Sub(staticEmbed, "static")
			if err == nil {
				staticHandler = api.StaticFS{Root: http.FS(sub)}
			} else {
				fmt.Fprintln(os.Stderr, "warning: no embedded UI (static/); API-only mode")
			}

			srv := api.New(store, inv, act, authSvc, dataDir, version, staticHandler)
			fmt.Fprintf(os.Stderr, "mock-me UI+API on %s (data=%s) — LAB/TEST/DEV ONLY\n", addr, dataDir)
			return api.ListenAndServe(addr, srv.Handler())
		},
	}
	cmd.Flags().StringVar(&dataDir, "data-dir", "./data", "persistent data directory for MockUps")
	cmd.Flags().StringVar(&addr, "listen", ":8080", "listen address")
	return cmd
}
