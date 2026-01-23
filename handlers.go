package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/akigithub888/chirpy/internal/auth"
	"github.com/akigithub888/chirpy/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) updateUserHandler(w http.ResponseWriter, r *http.Request) {
	// 🔐 Authenticate
	tokenString, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	userID, err := auth.ValidateJWT(tokenString, cfg.tokenSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// 📥 Decode body
	type updateUserRequest struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// 🚫 Require both fields
	if req.Email == "" || req.Password == "" {
		respondWithError(w, http.StatusBadRequest, "Email and password required")
		return
	}

	// 🔐 Hash password
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error hashing password")
		return
	}

	// 💾 Update user
	user, err := cfg.db.UpdateUser(r.Context(), database.UpdateUserParams{
		ID:             userID,
		Email:          req.Email,
		HashedPassword: hashedPassword,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating user")
		return
	}

	// ✅ Respond with updated user (no password)
	resp := struct {
		ID        uuid.UUID `json:"id"`
		Email     string    `json:"email"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}{
		ID:        user.ID,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	respondWithJSON(w, http.StatusOK, resp)
}

func (cfg *apiConfig) loginHandler(w http.ResponseWriter, r *http.Request) {
	//  Decode request body
	var req loginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	//  Look up user by email
	user, err := cfg.db.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}

	// Verify password
	ok, err := auth.CheckPasswordHash(req.Password, user.HashedPassword)
	if err != nil || !ok {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}

	//  Determine token expiration
	const maxExpiration = time.Hour
	expires := maxExpiration

	//  Create JWT
	token, err := auth.MakeJWT(user.ID, cfg.tokenSecret, expires)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not create token")
		return
	}

	refresh_token, err := auth.MakeRefreshToken()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not create refresh token")
		return
	}
	expiresAt := time.Now().UTC().Add(60 * 24 * time.Hour)

	_, err = cfg.db.CreateRefreshToken(
		r.Context(),
		database.CreateRefreshTokenParams{
			Token:     refresh_token,
			UserID:    user.ID,
			ExpiresAt: expiresAt,
		},
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not save refresh token")
		return
	}
	//  Respond with user + token
	resp := struct {
		ID           uuid.UUID `json:"id"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
		Email        string    `json:"email"`
		Token        string    `json:"token"`
		RefreshToken string    `json:"refresh_token"`
	}{
		ID:           user.ID,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		Email:        user.Email,
		Token:        token,
		RefreshToken: refresh_token,
	}

	respondWithJSON(w, http.StatusOK, resp)
}

func (cfg *apiConfig) refreshHandler(w http.ResponseWriter, r *http.Request) {
	tokenString, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	refreshTokenRecord, err := cfg.db.GetUserFromRefreshToken(r.Context(), tokenString)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if refreshTokenRecord.RevokedAt.Valid {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if time.Now().UTC().After(refreshTokenRecord.ExpiresAt) {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	accessToken, err := auth.MakeJWT(refreshTokenRecord.ID, cfg.tokenSecret, time.Hour)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not create access token")
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{
		"token": accessToken,
	})
}

func (cfg *apiConfig) revokeHandler(w http.ResponseWriter, r *http.Request) {
	tokenString, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	err = cfg.db.RevokeRefreshToken(r.Context(), tokenString)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not revoke refresh token")
		return
	}
	// StatusNoContent is 204 no content
	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) getChirpHandler(w http.ResponseWriter, r *http.Request) {
	chirpIDStr := r.PathValue("chirpID")

	chirpID, err := uuid.Parse(chirpIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid chirp ID")
		return
	}
	dbChirp, err := cfg.db.GetChirp(r.Context(), chirpID)
	if err != nil {
		if err == sql.ErrNoRows {
			respondWithError(w, http.StatusNotFound, "Chirp not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Error getting chirp")
		return
	}
	chirp := Chirp{
		ID:        dbChirp.ID,
		CreatedAt: dbChirp.CreatedAt,
		UpdatedAt: dbChirp.UpdatedAt,
		Body:      dbChirp.Body,
		UserID:    dbChirp.UserID,
	}
	respondWithJSON(w, http.StatusOK, chirp)
}

func (cfg *apiConfig) getChirpsHandler(w http.ResponseWriter, r *http.Request) {
	dbChirps, err := cfg.db.GetChirps(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting chips")
		return
	}
	chirps := make([]Chirp, 0, len(dbChirps))
	for _, dbChirp := range dbChirps {
		chirps = append(chirps, Chirp{
			ID:        dbChirp.ID,
			CreatedAt: dbChirp.CreatedAt,
			UpdatedAt: dbChirp.UpdatedAt,
			Body:      dbChirp.Body,
			UserID:    dbChirp.UserID,
		})
	}
	respondWithJSON(w, http.StatusOK, chirps)

}

func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, r *http.Request) {
	type createuserRequest struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	var req createuserRequest
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	if err := decoder.Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Error decoding user")
		return
	}
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error hashing password")
	}

	dbUser, err := cfg.db.CreateUser(r.Context(), database.CreateUserParams{
		Email:          req.Email,
		HashedPassword: hashedPassword,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating new user")
		return
	}
	user := User{
		ID:        dbUser.ID,
		Email:     dbUser.Email,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
	}
	respondWithJSON(w, http.StatusCreated, user)
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (cfg *apiConfig) createChirpHandler(w http.ResponseWriter, r *http.Request) {
	type createChirpRequest struct {
		Body string `json:"body"`
	}
	tokenString, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	userID, err := auth.ValidateJWT(tokenString, cfg.tokenSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req createChirpRequest
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	if err := decoder.Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Something went wrong")
		return
	}

	if len(req.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long")
		return
	}

	cleaned := cleanChirp(req.Body)

	dbChirp, err := cfg.db.CreateChirp(
		r.Context(),
		database.CreateChirpParams{
			Body:   cleaned,
			UserID: userID,
		},
	)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error creating chirp")
		return
	}

	chirp := Chirp{
		ID:        dbChirp.ID,
		CreatedAt: dbChirp.CreatedAt,
		UpdatedAt: dbChirp.UpdatedAt,
		Body:      dbChirp.Body,
		UserID:    dbChirp.UserID,
	}

	respondWithJSON(w, http.StatusCreated, chirp)
}

func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	hits := cfg.fileserverHits.Load()
	html := fmt.Sprintf(
		`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`,
		hits,
	)

	w.Write([]byte(html))
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {

	if cfg.platform != "dev" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	cfg.fileserverHits.Store(0)

	if err := cfg.db.DeleteAllRefreshTokens(r.Context()); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to delete tokens")
		return
	}

	if err := cfg.db.DeleteAllChirps(r.Context()); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to delete chirps")
		return
	}

	if err := cfg.db.DeleteAllUsers(r.Context()); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to delete users")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "all users deleted",
	})
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error": msg,
	})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

func cleanChirp(body string) string {
	badWords := map[string]struct{}{
		"kerfuffle": {},
		"sharbert":  {},
		"fornax":    {},
	}

	words := strings.Split(body, " ")

	for i, word := range words {
		lowered := strings.ToLower(word)
		if _, exists := badWords[lowered]; exists {
			words[i] = "****"
		}
	}

	return strings.Join(words, " ")
}
