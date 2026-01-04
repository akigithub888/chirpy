package main

import (
	"log"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func main() {
	cfg := &apiConfig{}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", readinessHandler)
	mux.HandleFunc("/metrics", cfg.metricsHandler)
	mux.HandleFunc("/reset", cfg.resetHandler)

	fileServer := http.FileServer(http.Dir("."))

	mux.Handle(
		"/app/",
		cfg.middlewareMetricsInc(
			http.StripPrefix("/app", fileServer),
		),
	)
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Println("Server running on http://localhost:8080")
	log.Fatal(server.ListenAndServe())

}
