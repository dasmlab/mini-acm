// Package netcfg writes HAProxy and dnsmasq lab fragments for compact clusters.
package netcfg

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/dasmlab/mini-mock/internal/config"
)

// WriteHAProxy emits an HAProxy config for API/MCS/ingress VIP frontends.
func WriteHAProxy(dir string, cfg *config.ClusterConfig) error {
	path := filepath.Join(dir, "haproxy.cfg")
	const tpl = `# mini-mock generated — bind VIPs on the provisioning host / gateway
global
  log /dev/log local0

defaults
  mode tcp
  timeout connect 10s
  timeout client  1m
  timeout server  1m

frontend api
  bind {{.APIVIP}}:6443
  default_backend api_backends

backend api_backends
  balance roundrobin
{{- range $i, $e := .Masters }}
  server master{{$i}} {{$e}}:6443 check
{{- end }}

frontend mcs
  bind {{.APIVIP}}:22623
  default_backend mcs_backends

backend mcs_backends
  balance roundrobin
{{- range $i, $e := .Masters }}
  server master{{$i}} {{$e}}:22623 check
{{- end }}

frontend ingress_http
  bind {{.IngressVIP}}:80
  default_backend ingress_http_backends

backend ingress_http_backends
  balance roundrobin
{{- range $i, $e := .Masters }}
  server master{{$i}} {{$e}}:80 check
{{- end }}

frontend ingress_https
  bind {{.IngressVIP}}:443
  default_backend ingress_https_backends

backend ingress_https_backends
  balance roundrobin
{{- range $i, $e := .Masters }}
  server master{{$i}} {{$e}}:443 check
{{- end }}
`
	masters := masterIPs(cfg)
	t, err := template.New("haproxy").Parse(tpl)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return t.Execute(f, map[string]interface{}{
		"APIVIP":     cfg.Network.APIVIP,
		"IngressVIP": cfg.Network.IngressVIP,
		"Masters":    masters,
	})
}

// WriteDNSMasq emits host records for api / api-int / apps.
func WriteDNSMasq(dir string, cfg *config.ClusterConfig) error {
	path := filepath.Join(dir, "dnsmasq.d-mini-mock.conf")
	content := fmt.Sprintf(`# mini-mock DNS for %s.%s
address=/api.%s.%s/%s
address=/api-int.%s.%s/%s
address=/.apps.%s.%s/%s
`,
		cfg.Cluster.Name, cfg.Cluster.BaseDomain,
		cfg.Cluster.Name, cfg.Cluster.BaseDomain, cfg.Network.APIVIP,
		cfg.Cluster.Name, cfg.Cluster.BaseDomain, cfg.Network.APIVIP,
		cfg.Cluster.Name, cfg.Cluster.BaseDomain, cfg.Network.IngressVIP,
	)
	masters := masterIPs(cfg)
	for i, ip := range masters {
		content += fmt.Sprintf("address=/%s-master-%d.%s.%s/%s\n",
			cfg.Cluster.Name, i, cfg.Cluster.Name, cfg.Cluster.BaseDomain, ip)
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func masterIPs(cfg *config.ClusterConfig) []string {
	// Mirror cluster.expandNodes IP arithmetic without importing provider.
	base := cfg.Nodes.IPBase
	out := make([]string, cfg.Nodes.Count)
	var a, b, c, d int
	_, _ = fmt.Sscanf(base, "%d.%d.%d.%d", &a, &b, &c, &d)
	for i := 0; i < cfg.Nodes.Count; i++ {
		out[i] = fmt.Sprintf("%d.%d.%d.%d", a, b, c, d+i)
	}
	return out
}
