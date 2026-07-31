package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dasmlab/mini-acm/internal/cluster"
	"github.com/dasmlab/mini-acm/internal/config"
	"github.com/dasmlab/mini-acm/internal/provider"
)

func newClusterCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Create / manage ACM compact managed clusters",
	}
	cmd.AddCommand(newClusterCreateCmd(gf))
	cmd.AddCommand(newClusterStatusCmd())
	cmd.AddCommand(newClusterDestroyCmd(gf))
	cmd.AddCommand(newClusterAttachISOCmd(gf))
	return cmd
}

func newClusterCreateCmd(gf *globalFlags) *cobra.Command {
	var (
		cfgPath string
		workDir string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create 3 VMs + DNS/HAProxy + ACM CR templates",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.LoadCluster(cfgPath)
			if err != nil {
				return err
			}
			p, err := provider.New(cfg.Provider.Type, provider.Options{
				DryRun: gf.dryRun,
				Manual: gf.manual,
			})
			if err != nil {
				return err
			}
			return cluster.Create(cmd.Context(), cfg, p, cluster.Options{
				DryRun:  gf.dryRun,
				Manual:  gf.manual,
				WorkDir: workDir,
			})
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "config/cluster.example.yaml", "cluster config YAML")
	cmd.Flags().StringVar(&workDir, "work-dir", "", "output dir for generated assets (default data/cluster-<name>)")
	return cmd
}

func newClusterStatusCmd() *cobra.Command {
	var cfgPath string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show planned nodes / VIP layout",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := config.LoadCluster(cfgPath)
			if err != nil {
				return err
			}
			return cluster.Status(cfg)
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "config/cluster.example.yaml", "cluster config YAML")
	return cmd
}

func newClusterDestroyCmd(gf *globalFlags) *cobra.Command {
	var (
		cfgPath string
		yes     bool
	)
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy cluster VMs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !yes && !gf.dryRun {
				return fmt.Errorf("refusing destroy without --yes")
			}
			cfg, err := config.LoadCluster(cfgPath)
			if err != nil {
				return err
			}
			p, err := provider.New(cfg.Provider.Type, provider.Options{
				DryRun: gf.dryRun,
				Manual: gf.manual,
			})
			if err != nil {
				return err
			}
			return cluster.Destroy(cmd.Context(), cfg, p)
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "config/cluster.example.yaml", "cluster config YAML")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm destroy")
	return cmd
}

func newClusterAttachISOCmd(gf *globalFlags) *cobra.Command {
	var (
		cfgPath string
		iso     string
	)
	cmd := &cobra.Command{
		Use:   "attach-iso",
		Short: "Attach discovery ISO to all cluster VMs and power on",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if iso == "" {
				return fmt.Errorf("--iso is required")
			}
			cfg, err := config.LoadCluster(cfgPath)
			if err != nil {
				return err
			}
			p, err := provider.New(cfg.Provider.Type, provider.Options{
				DryRun: gf.dryRun,
				Manual: gf.manual,
			})
			if err != nil {
				return err
			}
			return cluster.AttachAndBoot(cmd.Context(), cfg, p, iso)
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "config/cluster.example.yaml", "cluster config YAML")
	cmd.Flags().StringVar(&iso, "iso", "", "path to ACM InfraEnv discovery ISO")
	return cmd
}
