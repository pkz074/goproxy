package proxy

import (
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// Proxy forwards HTTP requests to a single upstream service.
type Proxy struct {
	upstream *url.URL
	handler  *httputil.ReverseProxy
}

// New creates a reverse proxy for one upstream URL.
func New(upstreamURL string) (*Proxy, error) {
	upstream, err := url.Parse(upstreamURL)
	if err != nil {
		return nil, err
	}

	if upstream.Scheme != "http" && upstream.Scheme != "https" {
		return nil, errors.New("upstream URL must use http or https")
	}

	if upstream.Host == "" {
		return nil, errors.New("upstream URL must include a host")
	}
	handler := &httputil.ReverseProxy{
		Rewrite: func(preq *httputil.ProxyRequest) {
			preq.SetURL(upstream)
			preq.Out.Host = upstream.Host
		},
	}

	handler.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}

	return &Proxy{
		upstream: upstream,
		handler:  handler,
	}, nil
}

// ServeHTTP forwards the request to the configured upstream.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.handler.ServeHTTP(w, r)
}

// Upstream returns the configured upstream URL.
func (p *Proxy) Upstream() *url.URL {
	return p.upstream
}
