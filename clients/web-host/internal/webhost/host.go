package webhost

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/takezoh/agent-grid/clients/goclient/clientconfig"
)

const (
	serverRoutePrefix     = "/host/servers/"
	protocolVersion       = "protocol-3"
	protocolVersionHeader = "X-Agent-Grid-Protocol-Version"
)

// Handler serves the shared SPA and one server-qualified transport for every
// configured client server. The immutable snapshot contains credentials; only
// its credential-free projection is exposed to browser JavaScript.
func Handler(snapshot clientconfig.Snapshot) (http.Handler, error) {
	// A fleet with nothing usable is a state the host must serve, not refuse.
	// Under per-entry admission an entry can become configuration-invalid on
	// its own, so "no usable server" is reachable from a configuration that
	// loaded — and refusing here would mean the same end state stops the host
	// on one route and not on another
	// (adr-20260814-server-display-name-naming-contract).
	distFS, err := DistFS()
	if err != nil {
		return nil, fmt.Errorf("webhost: embed sub-fs: %w", err)
	}

	mux := http.NewServeMux()
	fleet := newFleetRuntime(snapshot)
	mux.Handle("/", staticHandler(distFS))
	mux.HandleFunc("GET /host/config", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(snapshot.Projection()); err != nil {
			slog.Error("write host config projection", "err", err)
		}
	})
	mux.HandleFunc("/host/servers/", func(w http.ResponseWriter, r *http.Request) {
		serveServerRoute(snapshot, w, r)
	})
	mux.HandleFunc("GET /host/fleet", func(w http.ResponseWriter, r *http.Request) {
		serveFleet(fleet, w, r)
	})
	mux.HandleFunc("GET /host/fleet/health", func(w http.ResponseWriter, r *http.Request) {
		serveFleetHealth(fleet, w, r)
	})
	mux.HandleFunc("POST /host/fleet/servers/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/sessions"):
			serveFleetCreate(fleet, snapshot, w, r)
		case strings.HasSuffix(r.URL.Path, "/reconnect"):
			serveFleetReconnect(fleet, w, r)
		default:
			http.NotFound(w, r)
		}
	})
	return SecurityHeaders(mux), nil
}

func serveServerRoute(snapshot clientconfig.Snapshot, w http.ResponseWriter, r *http.Request) {
	serverID, upstreamPath, ok := parseServerRoute(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	server, found := snapshot.EnabledServer(serverID)
	if !found {
		http.Error(w, "unknown or disabled server", http.StatusNotFound)
		return
	}
	if upstreamPath == "/ws" && !sameOrigin(r) {
		http.Error(w, "forbidden origin", http.StatusForbidden)
		return
	}
	if upstreamPath != "/ws" && !strings.HasPrefix(upstreamPath, "/api/") {
		http.NotFound(w, r)
		return
	}
	target, err := url.Parse(server.URL)
	if err != nil {
		http.Error(w, "configured server is invalid", http.StatusInternalServerError)
		return
	}
	serverProxy(target, server, upstreamPath).ServeHTTP(w, r)
}

func parseServerRoute(path string) (serverID, upstreamPath string, ok bool) {
	if !strings.HasPrefix(path, serverRoutePrefix) {
		return "", "", false
	}
	remainder := strings.TrimPrefix(path, serverRoutePrefix)
	serverID, upstreamPath, found := strings.Cut(remainder, "/")
	if !found || serverID == "" {
		return "", "", false
	}
	return serverID, "/" + upstreamPath, true
}

func serverProxy(
	target *url.URL,
	server clientconfig.Server,
	upstreamPath string,
) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Transport: boundedTransport(),
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.URL.Path = joinTargetPath(target.Path, upstreamPath)
			pr.Out.URL.RawPath = ""
			pr.Out.Host = target.Host
			pr.Out.Header.Del("Origin")
			pr.Out.Header.Del("Authorization")
			pr.Out.Header.Set(protocolVersionHeader, protocolVersion)
			if server.Auth.Mode == "bearer" {
				pr.Out.Header.Set("Authorization", "Bearer "+server.Auth.Token)
			}
		},
		ModifyResponse: func(response *http.Response) error {
			if response.StatusCode >= http.StatusMultipleChoices &&
				response.StatusCode < http.StatusBadRequest {
				return errors.New("upstream redirect rejected")
			}
			return nil
		},
		ErrorHandler: proxyErrorHandler,
	}
}

func boundedTransport() http.RoundTripper {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: time.Second}).DialContext
	transport.ResponseHeaderTimeout = 10 * time.Second
	return transport
}

func joinTargetPath(base, path string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func proxyErrorHandler(w http.ResponseWriter, _ *http.Request, err error) {
	if isProxyUpstreamNotReady(err) {
		slog.Info("proxy: upstream not ready")
		w.Header().Set("Retry-After", "1")
		http.Error(w, "upstream not ready", http.StatusServiceUnavailable)
		return
	}
	slog.Error("proxy: request failed")
	http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
}

func isProxyUpstreamNotReady(err error) bool {
	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.EHOSTUNREACH) ||
		errors.Is(err, syscall.ENETUNREACH) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) &&
		(dnsErr.IsNotFound || dnsErr.IsTemporary || dnsErr.IsTimeout) {
		return true
	}
	var networkError net.Error
	return errors.As(err, &networkError) && networkError.Timeout()
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && strings.EqualFold(parsed.Host, r.Host)
}

func staticHandler(assets fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
