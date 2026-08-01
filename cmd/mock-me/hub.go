package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dasmlab/mock-me/internal/acm"
	"github.com/dasmlab/mock-me/internal/config"
	"github.com/dasmlab/mock-me/internal/hub"
	"github.com/dasmlab/mock-me/internal/provider"
)

func newHubCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hub",
		Short: "Bootstrap / manage the ACM management SNO",
	}
	cmd.AddCommand(newHubCreateCmd(gf))
	cmd.AddCommand(newHubStatusCmd())
	cmd.AddCommand(newHubDestroyCmd(gf))
	cmd.AddCommand(newHubInstallACMCmd(gf))
	return cmd
}

func newHubCreateCmd(gf *globalFlags) *cobra.Command {
	var (
		cfgPath    string
		pullSecret string
		sshKey     string
		skipWait   bool
		skipACM    bool
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create management SNO via local Agent-based Installer",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.LoadHub(cfgPath)
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
			return hub.Create(cmd.Context(), cfg, p, hub.Options{
				PullSecretPath: pullSecret,
				SSHKeyPath:     sshKey,
				DryRun:         gf.dryRun,
				Manual:         gf.manual,
				SkipWait:       skipWait,
				SkipACM:        skipACM,
			})
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "config/hub.example.yaml", "hub config YAML")
	cmd.Flags().StringVar(&pullSecret, "pull-secret", "", "path to pull-secret JSON (or $PULL_SECRET_FILE)")
	cmd.Flags().StringVar(&sshKey, "ssh-key", "", "path to SSH public key (or $SSH_PUBLIC_KEY_FILE)")
	cmd.Flags().BoolVar(&skipWait, "skip-wait", false, "do not wait for install-complete")
	cmd.Flags().BoolVar(&skipACM, "skip-acm", false, "do not install ACM after OCP")
	return cmd
}

func newHubStatusCmd() *cobra.Command {
	var cfgPath string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show hub workdir / kubeconfig hints",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := config.LoadHub(cfgPath)
			if err != nil {
				return err
			}
			return hub.Status(cfg)
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "config/hub.example.yaml", "hub config YAML")
	return cmd
}

func newHubDestroyCmd(gf *globalFlags) *cobra.Command {
	var (
		cfgPath string
		purge   bool
		yes     bool
	)
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy hub VM (and optionally workdir)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !yes && !gf.dryRun {
				return fmt.Errorf("refusing destroy without --yes")
			}
			cfg, err := config.LoadHub(cfgPath)
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
			return hub.Destroy(cmd.Context(), cfg, p, purge)
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "config/hub.example.yaml", "hub config YAML")
	cmd.Flags().BoolVar(&purge, "purge", false, "also delete workdir (ISO, auth, configs)")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm destroy")
	return cmd
}

func newHubInstallACMCmd(gf *globalFlags) *cobra.Command {
	var cfgPath string
	cmd := &cobra.Command{
		Use:   "install-acm",
		Short: "Apply MCE + ACM manifests to an existing hub",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.LoadHub(cfgPath)
			if err != nil {
				return err
			}
			kc := gf.kubeconfig
			if kc == "" {
				kc = cfg.Hub.WorkDir + "/auth/kubeconfig"
			}
			if gf.manual || gf.dryRun {
				return acm.PrintInstallInstructions(cfg.Hub.WorkDir)
			}
			return acm.Install(cmd.Context(), kc)
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "config/hub.example.yaml", "hub config YAML")
	return cmd
}
