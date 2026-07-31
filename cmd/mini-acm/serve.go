package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"github.com/dasmlab/mini-acm/internal/api"
	"github.com/dasmlab/mini-acm/internal/mockup"
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

  mini-acm serve --listen :8080 --data-dir ./data

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

			var staticHandler http.Handler
			sub, err := fs.Sub(staticEmbed, "static")
			if err == nil {
				staticHandler = api.StaticFS{Root: http.FS(sub)}
			} else {
				fmt.Fprintln(os.Stderr, "warning: no embedded UI (static/); API-only mode")
			}

			srv := api.New(store, dataDir, version, staticHandler)
			fmt.Fprintf(os.Stderr, "mini-acm UI+API on %s (data=%s) — LAB/TEST/DEV ONLY\n", addr, dataDir)
			return api.ListenAndServe(addr, srv.Handler())
		},
	}
	cmd.Flags().StringVar(&dataDir, "data-dir", "./data", "persistent data directory for MockUps")
	cmd.Flags().StringVar(&addr, "listen", ":8080", "listen address")
	return cmd
}
