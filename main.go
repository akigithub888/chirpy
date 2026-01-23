package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/akigithub888/chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
	tokenSecret    string
}
type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Expires  int    `json:"expires_in_seconds"`
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
		db:          dbQueries,
		platform:    os.Getenv("PLATFORM"),
		tokenSecret: os.Getenv("SECRET_KEY"),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/healthz", readinessHandler)
	mux.HandleFunc("GET /admin/metrics", cfg.metricsHandler)
	mux.HandleFunc("POST /admin/reset", cfg.resetHandler)
	mux.HandleFunc("POST /api/users", cfg.createUserHandler)
	mux.HandleFunc("POST /api/chirps", cfg.createChirpHandler)
	mux.HandleFunc("GET /api/chirps", cfg.getChirpsHandler)
	mux.HandleFunc("GET /api/chirps/{chirpID}", cfg.getChirpHandler)
	mux.HandleFunc("POST /api/login", cfg.loginHandler)
	mux.HandleFunc("POST /api/refresh", cfg.refreshHandler)
	mux.HandleFunc("POST /api/revoke", cfg.revokeHandler)

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
