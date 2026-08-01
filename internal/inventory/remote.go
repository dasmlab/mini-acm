package inventory

import (
	"encoding/base64"
	"fmt"
	"path"
	"strings"

	"golang.org/x/crypto/ssh"
)

// Dial opens an SSH client to the inventory host (lab — insecure host key).
func (s *Store) Dial(id string) (*ssh.Client, *MachineHost, error) {
	h, err := s.Get(id)
	if err != nil {
		return nil, nil, err
	}
	client, err := dialSSH(h)
	if err != nil {
		return nil, h, err
	}
	return client, h, nil
}

// RunScript executes a bash script on the host and returns combined output.
func (s *Store) RunScript(id, script string) (string, *MachineHost, error) {
	client, h, err := s.Dial(id)
	if err != nil {
		return "", h, err
	}
	defer client.Close()
	out, err := sshOutput(client, script)
	return out, h, err
}

// WriteRemoteFile writes content to a remote path (mkdir -p parent).
func (s *Store) WriteRemoteFile(id, remotePath string, content []byte) error {
	client, _, err := s.Dial(id)
	if err != nil {
		return err
	}
	defer client.Close()
	dir := path.Dir(remotePath)
	b64 := base64.StdEncoding.EncodeToString(content)
	script := fmt.Sprintf(`set -euo pipefail
mkdir -p %q
echo %q | base64 -d > %q
chmod 0644 %q
echo WRITE_OK=1
`, dir, b64, remotePath, remotePath)
	out, err := sshOutput(client, script)
	if err != nil {
		return fmt.Errorf("write %s: %v (%s)", remotePath, err, truncate(out, 400))
	}
	if !strings.Contains(out, "WRITE_OK=1") {
		return fmt.Errorf("write %s incomplete: %s", remotePath, truncate(out, 400))
	}
	return nil
}
