package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/dashhive/dashmsg"
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
func (s *server) routes(allowVoting bool) {
	// health check
	s.router.HandleFunc("/api/health", s.handleHealthCheck())

	// route to record incoming votes
	if allowVoting {
		s.router.HandleFunc("/api/vote", s.handleVote())
	} else {
		s.router.HandleFunc("/api/vote", s.handleVoteClosed())
	}

	s.router.HandleFunc("/api/candidates", s.handleCandidates())

	// audit routes
	if allowVoting {
		s.router.HandleFunc("/api/validVotes", isAuthorized(s.handleValidVotes()))
		s.router.HandleFunc("/api/allVotes", isAuthorized(s.handleAllVotes()))
	} else {
		// the public can view all votes once the voting has concluded
		s.router.HandleFunc("/api/validVotes", s.handleValidVotes())
		s.router.HandleFunc("/api/allVotes", s.handleAllVotes())
	}

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

// handleCandidates handles the candidates route
func (s *server) handleCandidates() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		candidates, err := GSheetToCandidates(s.gsheetKey)
		if err != nil {
			writeError(http.StatusInternalServerError, w, r)
			return
		}

		// Return response
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(candidates)
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
			writeErrorMessage("INVALID_NETWORK", http.StatusBadRequest, w, r)
			return
		}
		if err := dashmsg.MagicVerify(v.Address, []byte(v.Message), v.Signature); nil != err {
			writeErrorMessage("INVALID_SIGNATURE: "+err.Error(), http.StatusBadRequest, w, r)
		}

		// Insert vote
		err = s.db.Insert(&v)
		if err != nil {
			writeError(http.StatusInternalServerError, w, r)
			return
		}

		// Return response
		w.Header().Set("Content-Type", "application/json")
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
		w.Header().Set("Content-Type", "application/json")
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

		w.Header().Set("Content-Type", "application/json")
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

		w.Header().Set("Content-Type", "application/json")
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(JSONResult{
			Status:  http.StatusOK,
			Message: http.StatusText(http.StatusOK),
		})
	}
}

// JSONErrorMessage represents the JSON structure of an error message to be
// returned.
type JSONErrorMessage struct {
	Message string `json:"message,omitempty"`
	Status  int    `json:"status"`
	URL     string `json:"url"`
	Error   string `json:"error"`
}

// JSONResult represents the JSON structure of the success message to be
// returned.
type JSONResult struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

// writeErrorMessage returns a JSON error with a helpful message.
func writeErrorMessage(msg string, errorCode int, w http.ResponseWriter, r *http.Request) {
	result := JSONErrorMessage{
		Message: msg,
		Status:  errorCode,
		URL:     r.URL.Path,
		Error:   http.StatusText(errorCode),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(errorCode)
	_ = json.NewEncoder(w).Encode(result)
}

// writeError returns a generic JSON error blob.
func writeError(errorCode int, w http.ResponseWriter, r *http.Request) {
	msg := JSONErrorMessage{
		Status: errorCode,
		URL:    r.URL.Path,
		Error:  http.StatusText(errorCode),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(errorCode)
	_ = json.NewEncoder(w).Encode(msg)
}

// handleIndex is catch-all route handler.
func (s *server) handleIndex() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeError(http.StatusNotFound, w, r)
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
		if version != 0x4c && version != 0x10 {
			return false
		}
	case "testnet":
		if version != 0x8c && version != 0x13 {
			return false
		}
	default: // only mainnet and testnet supported for now
		return false
	}

	return len(decoded) == 20
}
