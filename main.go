package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type zonedTimestamp struct {
	time.Time
}

func (t *zonedTimestamp) UnmarshalJSON(buf []byte) error {
	//                           "2017-12-15T11:02:01.443-0500"
	tt, err := time.Parse("2006-01-02T15:04:05.999-0700", strings.Trim(string(buf), `"`))
	if err != nil {
		return err
	}
	t.Time = tt
	return nil
}

type User struct {
	UserName string `json:"name"`
	EmailAddress string `json:"emailAddress"`
	DisplayName string `json:"displayName"`
	TimeZone string `json:"timeZone"`
}

type HistoryItem struct {
	Field string `json:"field"`
	From string `json:"from"`
	FromString string `json:"fromString"`
	To string `json:"to"`
	ToString string `json:"toString"`
}

type History struct {
	Id string `json:"id"`
	Author User `json:"author"`
	Created zonedTimestamp `json:"created"`
	Items []HistoryItem `json:"items"`
}

type PagedChangelog struct {
	StartAt int `json:"startAt"`
	MaxResults int `json:"maxResults"`
	Total int `json:"total"`
	Histories []History `json:"histories"`
}

type Issue struct {
	Id string `json:"id"`
	Key string `json:"key"`
	Changelog PagedChangelog `json:"changelog"`
}

type PagedIssues struct {
	StartAt int `json:"startAt"`
	MaxResults int `json:"maxResults"`
	Total int `json:"total"`
	Issues []Issue `json:"issues"`
}

func main() {
	args := os.Args[1:]

	boardId := 2924
	if len(args) >= 1 {
		intValue, err := strconv.Atoi(args[0])
		if err == nil {
			boardId = intValue
		}
	}

	jiraUrl := os.Getenv("JIRA_URL")
	jiraUsername := os.Getenv("JIRA_USERNAME")
	jiraPassword := os.Getenv("JIRA_PASSWORD")

	if jiraUrl == "" {
		jiraUrl = "https://ultidev"
	}

	cl := &http.Client{
		// Disable TLS cert verification:
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	url := fmt.Sprintf("%s/rest/agile/1.0/board/%d/issue?fields=changelog&expand=changelog", jiraUrl, boardId)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.SetBasicAuth(jiraUsername, jiraPassword)

	rsp, err := cl.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	if rsp.StatusCode >= 300 {
		log.Fatalf("status = %d", rsp.StatusCode)
	}

	pagedIssues := &PagedIssues{}
	dec := json.NewDecoder(rsp.Body)
	err = dec.Decode(pagedIssues)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%+v\n", pagedIssues)
}