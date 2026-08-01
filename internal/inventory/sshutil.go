package inventory

import (
	"bytes"
	"strings"

	"golang.org/x/crypto/ssh"
)

// sshOutput runs script under bash -lc so PATH, redirects, and pipes work.
// Plain Session.Run without a shell returns empty stdout on some RHEL/sshd setups
// for systemctl/virsh — which made probes flap ready → partial after a successful Fix.
func sshOutput(client *ssh.Client, script string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	var buf bytes.Buffer
	session.Stdout = &buf
	session.Stderr = &buf
	q := "'" + strings.ReplaceAll(script, "'", `'\''`) + "'"
	if err := session.Run("bash -lc " + q); err != nil {
		return strings.TrimSpace(buf.String()), err
	}
	return strings.TrimSpace(buf.String()), nil
}

func mustActive(out string) bool {
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "active" {
			return true
		}
	}
	return false
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}
