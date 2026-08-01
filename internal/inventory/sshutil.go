package inventory

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// sshOutput runs script via bash on stdin (same reliable pattern as Fix).
func sshOutput(client *ssh.Client, script string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return "", err
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		return "", err
	}

	if err := session.Start("bash --noprofile --norc -s"); err != nil {
		return "", err
	}

	var (
		buf bytes.Buffer
		wg  sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&buf, io.MultiReader(stdout, stderr))
	}()

	_, _ = io.WriteString(stdin, strings.TrimSpace(script)+"\n")
	_ = stdin.Close()
	err = session.Wait()
	wg.Wait()
	return strings.TrimSpace(buf.String()), err
}

// sshOutputRetry re-runs on empty/incomplete output (seen intermittently over WG SSH).
func sshOutputRetry(client *ssh.Client, script string, attempts int) (string, error) {
	if attempts < 1 {
		attempts = 1
	}
	var out string
	var err error
	for i := 0; i < attempts; i++ {
		out, err = sshOutput(client, script)
		if err == nil && strings.Contains(out, "PROBE_OK=1") {
			return out, nil
		}
		time.Sleep(time.Duration(200*(i+1)) * time.Millisecond)
	}
	if err == nil && out == "" {
		err = fmt.Errorf("empty remote probe output after %d attempts", attempts)
	} else if err == nil {
		err = fmt.Errorf("incomplete remote probe output after %d attempts", attempts)
	}
	return out, err
}

func mustActive(out string) bool {
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "active" {
			return true
		}
	}
	return false
}
