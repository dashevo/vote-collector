package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/go-pg/pg"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// server is an object which implements the http.Handler interface (passes to
// router) and related connection objects hang off it (e.g. db conn)
type server struct {
	router mux.Router
	db     *pg.DB
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

	// create a server object and add db connection
	srv := server{
		db: db,
	}
	srv.routes()

	// allow CORS w/mux router
	headersOk := handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type"})
	originsOk := handlers.AllowedOrigins([]string{"*"})
	methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS"})

	// serve the API
	listenAt := os.Getenv("BIND_HOST") + ":" + os.Getenv("BIND_PORT")
	fmt.Printf("%s listening at %s\n", os.Args[:1], listenAt)
	http.ListenAndServe(listenAt, handlers.CORS(originsOk, headersOk, methodsOk)(srv))
}
