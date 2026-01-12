package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/akigithub888/chirpy/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal(".env not loaded")
	}
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("Unable to open a connection to database")
	}
	dbQueries := database.New(db)
	cfg := &apiConfig{
		db: dbQueries,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/healthz", readinessHandler)
	mux.HandleFunc("GET /admin/metrics", cfg.metricsHandler)
	mux.HandleFunc("POST /admin/reset", cfg.resetHandler)
	mux.HandleFunc("POST /api/validate_chirp", cfg.validateChirpHandler)

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
