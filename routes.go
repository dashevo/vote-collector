package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/btcsuite/btcutil/base58"
	jwt "github.com/dgrijalva/jwt-go"
)

// JWTSecretKey is used to verify the JWT was signed w/the same, used for
// authorization.
// See also: https://jwt.io/#debugger
var JWTSecretKey []byte

// DashNetwork is used for validating the address network byte
var DashNetwork string

// ServeHTTP passes requests thru to the router.
func (s server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// routes defines the routes the server will handle
func (s *server) routes() {
	// health check
	s.router.HandleFunc("/health", s.handleHealthCheck())

	// route to record incoming votes
	s.router.HandleFunc("/vote", s.handleVoteClosed())

	// audit routes
	s.router.HandleFunc("/validVotes", isAuthorized(s.handleValidVotes()))
	s.router.HandleFunc("/allVotes", isAuthorized(s.handleAllVotes()))

	// TODO: catch-all (404)
	s.router.PathPrefix("/").Handler(s.handleIndex())
}

// isAuthorized is used to wrap handlers that need authz
func isAuthorized(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bearerToken, ok := r.Header["Authorization"]
		if !ok {
			writeError(http.StatusUnauthorized, w, r)
			return
		}

		// strip the "Bearer " from the beginning
		actualTokenStr := strings.TrimPrefix(bearerToken[0], "Bearer ")

		// Parse and validate token from request header
		token, err := jwt.Parse(actualTokenStr, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return "invalid signing method", nil
			}
			return JWTSecretKey, nil
		})
		if err != nil {
			writeError(http.StatusUnauthorized, w, r)
			return
		}

		// JWT is valid, pass the request thru to protected route
		if token.Valid {
			f(w, r)
		}
	}
}

// handleVote handles the vote route
func (s *server) handleVote() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse vote body
		var v Vote
		err := json.NewDecoder(r.Body).Decode(&v)
		if err != nil {
			writeError(http.StatusBadRequest, w, r)
			return
		}
		v.CreatedAt = time.Now().UTC()

		// Very basic input validation. In the future the ideal solution would
		// be to validate signature as well.
		if !isValidAddress(v.Address, os.Getenv("DASH_NETWORK")) {
			writeError(http.StatusBadRequest, w, r)
			return
		}

		// Insert vote
		err = s.db.Insert(&v)
		if err != nil {
			writeError(http.StatusInternalServerError, w, r)
			return
		}

		// Return response
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(JSONResult{
			Status:  http.StatusCreated,
			Message: "Vote Recorded",
		})
	}
}

// handleVoteClosed handles the vote route once voting is Closed
func (s *server) handleVoteClosed() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Return response
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(JSONResult{
			Status:  http.StatusForbidden,
			Message: "Voting Closed",
		})
	}
}

// handleValidVotes is the route for vote tallying, and returns only most
// current vote per MN collateral address.
// TODO: consider pagination if this gets too big.
func (s *server) handleValidVotes() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		votes, err := getCurrentVotesOnly(s.db)
		if err != nil {
			writeError(http.StatusInternalServerError, w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		err = json.NewEncoder(w).Encode(&votes)
		if err != nil {
			writeError(http.StatusInternalServerError, w, r)
			return
		}
	}
}

// handleAllVotes is the route for listing all votes, including old, superceded
// ones. Use with caution! (For audit purposes only.)
// TODO: consider pagination if this gets too big.
func (s *server) handleAllVotes() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		votes, err := getAllVotes(s.db)
		if err != nil {
			writeError(http.StatusInternalServerError, w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		err = json.NewEncoder(w).Encode(&votes)
		if err != nil {
			writeError(http.StatusInternalServerError, w, r)
			return
		}
	}
}

// handleHealthCheck handles the health check route, an unauthenticated route
// needed for load balancers to know this service is still "healthy".
func (s *server) handleHealthCheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(JSONResult{
			Status:  http.StatusOK,
			Message: http.StatusText(http.StatusOK),
		})
	}
}

// JSONErrorMessage represents the JSON structure of an error message to be
// returned.
type JSONErrorMessage struct {
	Status int    `json:"status"`
	URL    string `json:"url"`
	Error  string `json:"error"`
}

// JSONResult represents the JSON structure of the success message to be
// returned.
type JSONResult struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

// writeError returns a generic JSON error blob.
func writeError(errorCode int, w http.ResponseWriter, r *http.Request) {
	msg := JSONErrorMessage{
		Status: errorCode,
		URL:    r.URL.Path,
		Error:  http.StatusText(errorCode),
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(errorCode)
	_ = json.NewEncoder(w).Encode(msg)
	return
}

// handleIndex is catch-all route handler.
func (s *server) handleIndex() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeError(http.StatusNotFound, w, r)
		return
	}
}

func init() {
	JWTSecretKey = []byte(os.Getenv("JWT_SECRET_KEY"))
}

// helper methods

// isValidAddress checks if a given string is a valid Dash address
func isValidAddress(addr string, dashNetwork string) bool {
	decoded, version, err := base58.CheckDecode(addr)
	if err != nil {
		return false
	}

	switch dashNetwork {
	case "mainnet":
		if version != 76 && version != 16 {
			return false
		}
	case "testnet":
		if version != 140 && version != 19 {
			return false
		}
	default: // only mainnet and testnet supported for now
		return false
	}

	if len(decoded) != 20 {
		return false
	}

	return true
}
