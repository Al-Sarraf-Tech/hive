// Package proxy manages ingress load balancer containers for Hive services.
// When a service opts into ingress via [service.X.ingress] in the Hivefile,
// the proxy manager creates an nginx container that distributes traffic
// across all healthy replicas with automatic failover.
package proxy

import (
	"bytes"
	"text/template"
)

// Upstream represents one backend replica endpoint for the load balancer.
type Upstream struct {
	Addr   string // e.g. "love-note-web-0:80" (docker DNS) or "192.168.50.5:26149" (host)
	Weight int    // 0 = equal weight (default), >0 for canary deploys
}

var nginxTmpl = template.Must(template.New("nginx").Parse(`
worker_processes 1;
error_log /dev/stderr warn;
pid /tmp/nginx.pid;

events {
    worker_connections 256;
}

http {
    access_log /dev/stdout;

    upstream {{ .ServiceName }} {
{{- range .Upstreams }}
        server {{ .Addr }}{{ if .Weight }} weight={{ .Weight }}{{ end }};
{{- end }}
{{- if not .Upstreams }}
        server 127.0.0.1:1 down;
{{- end }}
    }

    server {
        listen {{ .ListenPort }};

        location / {
            proxy_pass http://{{ .ServiceName }};
            proxy_connect_timeout 5s;
            proxy_read_timeout 60s;
            proxy_send_timeout 30s;
            proxy_next_upstream error timeout http_502 http_503;
            proxy_next_upstream_tries 3;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
        }

        location /hive-ingress-health {
            return 200 "ok";
            add_header Content-Type text/plain;
        }
    }
}
`))

var nginxTLSTmpl = template.Must(template.New("nginx-tls").Parse(`
worker_processes 1;
error_log /dev/stderr warn;
pid /tmp/nginx.pid;

events {
    worker_connections 256;
}

http {
    access_log /dev/stdout;

    upstream {{ .ServiceName }} {
{{- range .Upstreams }}
        server {{ .Addr }}{{ if .Weight }} weight={{ .Weight }}{{ end }};
{{- end }}
{{- if not .Upstreams }}
        server 127.0.0.1:1 down;
{{- end }}
    }

    server {
        listen {{ .ListenPort }} ssl;
        ssl_certificate /etc/nginx/certs/tls.crt;
        ssl_certificate_key /etc/nginx/certs/tls.key;
        ssl_protocols TLSv1.2 TLSv1.3;
        ssl_prefer_server_ciphers on;

        location / {
            proxy_pass http://{{ .ServiceName }};
            proxy_connect_timeout 5s;
            proxy_read_timeout 60s;
            proxy_send_timeout 30s;
            proxy_next_upstream error timeout http_502 http_503;
            proxy_next_upstream_tries 3;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto https;
        }

        location /hive-ingress-health {
            return 200 "ok";
            add_header Content-Type text/plain;
        }
    }
}
`))

type nginxData struct {
	ServiceName string
	ListenPort  int
	Upstreams   []Upstream
}


// safeNginxName validates that a string is safe for use in nginx config.
// Rejects any string containing characters that could inject nginx directives.
func safeNginxName(s string) bool {
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == ':') {
			return false
		}
	}
	return len(s) > 0
}

// GenerateNginxConf renders an nginx.conf for the given service.
// listenPort is the port nginx listens on inside the container (always 80).
// upstreams is the list of healthy backend addresses.
// When upstreams is empty, a placeholder down server is used so nginx can
// still start — requests will get 502 until a backend recovers.
// GenerateNginxTLSConf renders an nginx.conf with TLS termination for the given service.
func GenerateNginxTLSConf(serviceName string, listenPort int, upstreams []Upstream) []byte {
	if !safeNginxName(serviceName) {
		serviceName = "default"
	}
	var safe []Upstream
	for _, u := range upstreams {
		if safeNginxName(u.Addr) {
			safe = append(safe, u)
		}
	}
	var buf bytes.Buffer
	if err := nginxTLSTmpl.Execute(&buf, nginxData{
		ServiceName: serviceName,
		ListenPort:  listenPort,
		Upstreams:   safe,
	}); err != nil {
		return []byte("# template error: " + err.Error())
	}
	return buf.Bytes()
}

func GenerateNginxConf(serviceName string, listenPort int, upstreams []Upstream) []byte {
	// Validate service name is safe for nginx config
	if !safeNginxName(serviceName) {
		serviceName = "default"
	}
	// Filter unsafe upstream addresses to prevent nginx config injection
	var safe []Upstream
	for _, u := range upstreams {
		if safeNginxName(u.Addr) {
			safe = append(safe, u)
		}
	}
	upstreams = safe
	var buf bytes.Buffer
	if err := nginxTmpl.Execute(&buf, nginxData{
		ServiceName: serviceName,
		ListenPort:  listenPort,
		Upstreams:   upstreams,
	}); err != nil {
		return []byte("# template error: " + err.Error())
	}
	return buf.Bytes()
}
