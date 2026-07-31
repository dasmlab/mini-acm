// Package acm installs Multicluster Engine + Advanced Cluster Management operators.
package acm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// Install applies bundled MCE/ACM manifests using oc against kubeconfig.
func Install(ctx context.Context, kubeconfig string) error {
	if kubeconfig == "" {
		return fmt.Errorf("kubeconfig required for ACM install")
	}
	dir, err := manifestsDir()
	if err != nil {
		return err
	}
	files := []string{
		filepath.Join(dir, "namespace.yaml"),
		filepath.Join(dir, "mce-operator.yaml"),
		filepath.Join(dir, "mch.yaml"),
	}
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			fmt.Fprintf(os.Stderr, "missing %s — printing instructions instead\n", f)
			return PrintInstallInstructions(filepath.Dir(kubeconfig))
		}
		if err := oc(ctx, kubeconfig, "apply", "-f", f); err != nil {
			return err
		}
	}
	fmt.Println("ACM manifests applied. Waiting briefly for MCH to settle is operator-dependent;")
	fmt.Println("check: oc get mch -A ; oc get csv -n open-cluster-management")
	time.Sleep(2 * time.Second)
	_ = oc(ctx, kubeconfig, "get", "mch", "-A")
	return nil
}

// PrintInstallInstructions emits manual ACM install steps.
func PrintInstallInstructions(workDir string) error {
	fmt.Println()
	fmt.Println("=== ACM install (manual) ===")
	fmt.Println("1. export KUBECONFIG=<hub>/auth/kubeconfig")
	fmt.Println("2. oc apply -f manifests/acm/namespace.yaml")
	fmt.Println("3. oc apply -f manifests/acm/mce-operator.yaml")
	fmt.Println("4. Wait for MulticlusterEngine Available")
	fmt.Println("5. oc apply -f manifests/acm/mch.yaml")
	fmt.Println("6. oc get mch -n open-cluster-management -w")
	if workDir != "" {
		fmt.Printf("hint workdir: %s\n", workDir)
	}
	return nil
}

func manifestsDir() (string, error) {
	// Prefer repo-relative manifests/acm when running from source tree.
	candidates := []string{
		"manifests/acm",
		filepath.Join("..", "manifests", "acm"),
	}
	_, file, _, ok := runtime.Caller(0)
	if ok {
		candidates = append(candidates, filepath.Join(filepath.Dir(file), "..", "..", "manifests", "acm"))
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("manifests/acm not found")
}

func oc(ctx context.Context, kubeconfig string, args ...string) error {
	if _, err := exec.LookPath("oc"); err != nil {
		return fmt.Errorf("oc not on PATH: %w", err)
	}
	cmd := exec.CommandContext(ctx, "oc", args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfig)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
