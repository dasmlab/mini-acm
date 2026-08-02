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

// ReadRemoteFile returns the contents of a remote file (nil if missing).
func (s *Store) ReadRemoteFile(id, remotePath string) ([]byte, error) {
	client, _, err := s.Dial(id)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	script := fmt.Sprintf(`set -euo pipefail
if [ ! -f %q ]; then
  echo MISSING=1
  exit 0
fi
echo CONTENT_B64_BEGIN
base64 -w0 %q 2>/dev/null || base64 %q
echo
echo CONTENT_B64_END
`, remotePath, remotePath, remotePath)
	out, err := sshOutput(client, script)
	if err != nil {
		return nil, fmt.Errorf("read %s: %v (%s)", remotePath, err, truncate(out, 400))
	}
	if strings.Contains(out, "MISSING=1") {
		return nil, nil
	}
	start := strings.Index(out, "CONTENT_B64_BEGIN")
	end := strings.Index(out, "CONTENT_B64_END")
	if start < 0 || end < 0 || end <= start {
		return nil, fmt.Errorf("read %s: malformed response", remotePath)
	}
	payload := strings.TrimSpace(out[start+len("CONTENT_B64_BEGIN") : end])
	b, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("read %s: decode: %w", remotePath, err)
	}
	return b, nil
}
