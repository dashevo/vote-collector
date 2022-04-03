package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-pg/pg"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"github.com/joho/godotenv"
)

// server is an object which implements the http.Handler interface (passes to
// router) and related connection objects hang off it (e.g. db conn)
type server struct {
	router              mux.Router
	db                  *pg.DB
	gsheetKey           string
	candidatesUpdateMux *sync.Mutex
	candidatesMux       *sync.RWMutex
	candidates          []Candidate
	candidatesUpdatedAt time.Time
	votingStart         time.Time
	votingEnd           time.Time
}

// envCheck is called upon startup to ensure the required environment variables
// are set
func envCheck() {
	// ensure config vars set
	reqd := []string{
		"PGUSER",
		"PGHOST",
		"PGPORT",
		"PGPASSWORD",
		"PGDATABASE",
		"VOTING_START_DATE",
		"VOTING_END_DATE",
		"GSHEET_KEY",
		"JWT_SECRET_KEY",
		"DASH_NETWORK",
		"BIND_HOST",
		"BIND_PORT",
	}
	missing := false
	for _, env := range reqd {
		val, ok := os.LookupEnv(env)
		if !ok || (len(val) == 0) {
			missing = true
			fmt.Fprintf(os.Stderr, "error: required env var %s not set\n", env)
		}
	}
	if missing {
		os.Exit(1)
	}

	if val := os.Getenv("DASH_NETWORK"); val != "testnet" && val != "mainnet" {
		fmt.Fprintf(os.Stderr, "error: unknown Dash network '%s'\n", val)
		fmt.Fprintf(os.Stderr, "\texpected \"mainnet\" or \"testnet\"\n")
		os.Exit(1)
	}
}

func main() {
	_ = godotenv.Load(".env")
	_ = godotenv.Load(".env.secret")

	envCheck()

	// create a PG database connection
	db := pg.Connect(&pg.Options{
		User:     os.Getenv("PGUSER"),
		Addr:     os.Getenv("PGHOST") + ":" + os.Getenv("PGPORT"),
		Password: os.Getenv("PGPASSWORD"),
		Database: os.Getenv("PGDATABASE"),
	})
	defer db.Close()

	// create the database tables if they don't exist
	err := createSchema(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	votingStart, err := time.Parse(time.RFC3339, os.Getenv("VOTING_START_DATE"))
	if nil != err {
		fmt.Fprintf(
			os.Stderr,
			"error parsing 'VOTING_START_DATE': %q: %s\n",
			os.Getenv("VOTING_START_DATE"),
			err,
		)
		os.Exit(1)
	}
	votingEnd, err := time.Parse(time.RFC3339, os.Getenv("VOTING_END_DATE"))
	if nil != err {
		fmt.Fprintf(
			os.Stderr,
			"error parsing 'VOTING_END_DATE': %q: %s\n",
			os.Getenv("VOTING_END_DATE"),
			err,
		)
		os.Exit(1)
	}

	gsheetKey := os.Getenv("GSHEET_KEY")

	// create a server object and add db connection
	srv := server{
		db:                  db,
		gsheetKey:           gsheetKey,
		votingStart:         votingStart,
		votingEnd:           votingEnd,
		candidatesMux:       &sync.RWMutex{},
		candidatesUpdateMux: &sync.Mutex{},
	}

	err = srv.updateCandidates()
	if nil != err {
		fmt.Fprintf(os.Stderr, "error parsing candidates from CSV using 'GSHEET_KEY': %s\n", err)
		os.Exit(1)
	}

	srv.routes()

	// allow CORS w/mux router
	headersOk := handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type"})
	originsOk := handlers.AllowedOrigins([]string{"*"})
	methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS"})

	// serve the API
	listenAt := os.Getenv("BIND_HOST") + ":" + os.Getenv("BIND_PORT")
	fmt.Printf("%s listening at %s\n", os.Args[:1], listenAt)
	if err := http.ListenAndServe(listenAt, handlers.CORS(originsOk, headersOk, methodsOk)(srv)); nil != err {
		log.Fatal(err)
	}
}
