package traffic_inspector

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

func newReverseProxy(upstream *url.URL, transport *http.Transport) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.Transport = transport
	proxy.FlushInterval = -1 // Flush each write so streaming responses reach the client promptly.
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = upstream.Scheme
		req.URL.Host = upstream.Host
		req.Host = upstream.Host
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}
	return proxy
}
