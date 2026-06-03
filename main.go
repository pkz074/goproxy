package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/pkz074/goproxy/internal/proxy"
)

func main() {
	listenAddr := flag.String("listen", ":8080", "address the proxy listens on")
	upstreamURL := flag.String("upstream", "", "upstream service URL, for example http://localhost:8081")
	flag.Parse()

	if *upstreamURL == "" {
		log.Fatal("missing required -upstream URL")
	}

	handler, err := proxy.New(*upstreamURL)
	if err != nil {
		log.Fatalf("invalid upstream URL: %v", err)
	}

	server := &http.Server{
		Addr:    *listenAddr,
		Handler: handler,
	}

	log.Printf("goproxy listening on %s and forwarding to %s", *listenAddr, handler.Upstream())
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("proxy server failed: %v", err)
	}
}
