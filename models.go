package main

import (
	"fmt"
	"time"

	"github.com/go-pg/pg"
	"github.com/go-pg/pg/orm"
)

// Vote represents a MNO vote object and is a model for the ORM.
type Vote struct {
	// Unique indexes are ideal, but add more logic to the program. So we'll leave this dumb and
	// use a query to pull the most recent votes (only) per MN address instead.
	Address   string    `json:"addr"`
	Message   string    `json:"msg"`
	Signature string    `json:"sig"`
	CreatedAt time.Time `json:"ts"`
}

// String implements the Stringer interface for Vote
func (v Vote) String() string {
	return fmt.Sprintf(
		"Vote<%s %s %s %v>",
		v.Address,
		v.Message,
		v.Signature,
		v.CreatedAt.UTC().Format(time.RFC3339),
	)
}

// createSchema makes the DB tables if they don't exist
func createSchema(db *pg.DB) error {
	for _, model := range []interface{}{(*Vote)(nil)} {
		err := db.CreateTable(model, &orm.CreateTableOptions{
			IfNotExists: true,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// getCurrentVotesOnly returns a list of the latest votes for each address
func getCurrentVotesOnly(db *pg.DB) ([]Vote, error) {
	countingVotes := []Vote{}

	query := `
	select distinct t.address
	     , t.message
	     , t.signature
	     , t.created_at
	  from votes t
	 inner join (
	       select address
	            , max(created_at) as maxdate
	         from votes
	        group by address
	       ) tm
	    on t.address = tm.address
	   and t.created_at = tm.maxdate
	`

	_, err := db.Query(&countingVotes, query)
	return countingVotes, err
}

// getAllVotes returns a list of all votes in the database
func getAllVotes(db *pg.DB) ([]Vote, error) {
	votes := []Vote{}

	err := db.Model(&votes).Select()
	return votes, err
}
