package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

// Candidate has a Name and Nickname
type Candidate struct {
	// alias, text, value, key
	Name           string `json:"name"`
	Handle         string `json:"handle"`
	Email          string `json:"email"`
	Link           string `json:"link"`
	TrustProtector bool   `json:"trust_protector"`
}

// GSheetToCandidates converts our special Google Sheet to JSON
func GSheetToCandidates(sheetKey string) ([]Candidate, error) {
	gsurl := fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/export?format=csv&usp=sharing", sheetKey)
	resp, err := http.Get(gsurl)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("response failed with status code: %q and body: %q", resp.StatusCode, body)
	}

	csvr := csv.NewReader(resp.Body)
	headerRow, err := csvr.Read()
	if nil != err {
		if io.EOF == err {
			return nil, err
		}
	}

	handleIndex := -1
	nameIndex := -1
	emailIndex := -1
	approvedIndex := -1
	currentTPIndex := -1
	profileLinkIndex := -1

	for i, v := range headerRow {
		lower := strings.ToLower(v)

		if handleIndex < 0 && (strings.Contains(lower, "pseudonym") ||
			strings.Contains(lower, "handle") ||
			strings.Contains(lower, "nick")) {
			handleIndex = i
			continue
		}

		if emailIndex < 0 && (strings.Contains(lower, "email") ||
			strings.Contains(lower, "e-mail")) {
			emailIndex = i
			continue
		}

		if nameIndex < 0 && (strings.Contains(lower, "full name") ||
			strings.Contains(lower, "legal name") ||
			strings.Contains(lower, "name")) {
			nameIndex = i
			continue
		}

		if approvedIndex < 0 && "approved" == lower ||
			"allowed" == lower ||
			"certified" == lower {
			approvedIndex = i
			continue
		}

		if currentTPIndex < 0 &&
			strings.Contains(lower, "current tp") {
			currentTPIndex = i
			continue
		}

		if profileLinkIndex < 0 &&
			strings.Contains(lower, "profile link") {
			profileLinkIndex = i
			continue
		}

		if approvedIndex >= 0 &&
			emailIndex >= 0 &&
			handleIndex >= 0 &&
			profileLinkIndex >= 0 &&
			currentTPIndex >= 0 &&
			nameIndex >= 0 {
			break
		}
	}

	missing := []string{}
	if approvedIndex < 0 {
		missing = append(missing, "approved")
	}
	if emailIndex < 0 {
		missing = append(missing, "email")
	}
	if handleIndex < 0 {
		missing = append(missing, "handle")
	}
	if nameIndex < 0 {
		missing = append(missing, "name")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("sheet is missing required columns: %s", strings.Join(missing, ","))
	}

	candidates := []Candidate{}
	for {
		row, err := csvr.Read()
		if nil != err {
			if io.EOF == err {
				break
			}
			// could be ErrFieldCount, which we're not being lax about
			return nil, err
		}

		approved := strings.ToLower(row[approvedIndex])
		fmt.Println("approved:", approved)
		if "y" == approved || "yes" == approved {
			c := Candidate{
				Handle: row[handleIndex],
				Name:   row[nameIndex],
				Email:  row[emailIndex],
			}
			if currentTPIndex >= 0 {
				currentTP := strings.ToLower(row[currentTPIndex])
				if "y" == currentTP || "yes" == currentTP {
					c.TrustProtector = true
				}
			}
			if profileLinkIndex >= 0 {
				profileLink := row[profileLinkIndex]
				c.Link = profileLink
			}
			candidates = append(candidates, c)
		}
	}

	return candidates, nil
}
