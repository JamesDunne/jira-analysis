package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
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
	UserName     string `json:"name"`
	EmailAddress string `json:"emailAddress"`
	DisplayName  string `json:"displayName"`
	TimeZone     string `json:"timeZone"`
}

type HistoryItem struct {
	Field      string `json:"field"`
	From       string `json:"from"`
	FromString string `json:"fromString"`
	To         string `json:"to"`
	ToString   string `json:"toString"`
}

type History struct {
	Id      string         `json:"id"`
	Author  User           `json:"author"`
	Created zonedTimestamp `json:"created"`
	Items   []HistoryItem  `json:"items"`
}

type PagedChangelog struct {
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	Histories  []History `json:"histories"`
}

type Issue struct {
	Id        string         `json:"id"`
	Key       string         `json:"key"`
	Changelog PagedChangelog `json:"changelog"`

	// computed:
	Status             string
	StatusTime         time.Time
	Assigned           User
	StatusBusinessDays int
}

type PagedIssues struct {
	StartAt    int     `json:"startAt"`
	MaxResults int     `json:"maxResults"`
	Total      int     `json:"total"`
	Issues     []Issue `json:"issues"`
}

type Date struct {
	time.Time
}

func dateOf(t time.Time) Date {
	y, m, d := t.Date()
	l := t.Location()
	return Date{time.Date(y, m, d, 0, 0, 0, 0, l)}
}

func (date Date) NextDate() Date {
	return dateOf(date.Time.Add(25 * time.Hour))
}

func (date Date) BusinessDaysUntil(until Date) int {
	d := date

	// Count weekdays, skipping weekends:
	days := 0
	for d.Time.Before(until.Time) {
		if d.Time.Weekday() == time.Saturday {
			d = d.NextDate()
		}
		if d.Time.Weekday() == time.Sunday {
			d = d.NextDate()
		}

		if d.Time.Before(until.Time) {
			d = d.NextDate()
			days++
		}
	}

	return days
}

func cachedGet(cacheFilename string, url string, cl *http.Client) (issuesJsonBody io.ReadCloser, err error) {
	stat, statErr := os.Stat(cacheFilename)

	cacheHit := statErr == nil || !os.IsNotExist(statErr)
	if cacheHit && stat != nil {
		if stat.ModTime().Before(time.Now().Add(-time.Hour)) {
			cacheHit = false
		}
	}

	if cacheHit {
		b, err := ioutil.ReadFile(cacheFilename)
		if err != nil {
			cacheHit = false
		} else {
			issuesJsonBody = ioutil.NopCloser(bytes.NewReader(b))
			return issuesJsonBody, nil
		}
	}

	if !cacheHit {
		var req *http.Request
		req, err = http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			log.Fatal(err)
		}
		req.SetBasicAuth(os.ExpandEnv("$JIRA_USERNAME"), os.ExpandEnv("$JIRA_PASSWORD"))

		var rsp *http.Response
		rsp, err = cl.Do(req)
		if err != nil {
			return nil, err
		}
		if rsp.StatusCode >= 300 {
			//log.Fatalf("status = %d", rsp.StatusCode)
			return nil, errors.Errorf("HTTP response %s", rsp.Status)
		}

		var b []byte
		b, err = ioutil.ReadAll(rsp.Body)
		if err != nil {
			return nil, err
		}
		rsp.Body.Close()

		// cache response in file:
		ioutil.WriteFile(cacheFilename, b, 0600)

		issuesJsonBody = ioutil.NopCloser(bytes.NewReader(b))

		return issuesJsonBody, nil
	}

	return
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

	if os.Getenv("JIRA_URL") == "" {
		os.Setenv("JIRA_URL", "https://ultidev")
	}

	cl := &http.Client{
		// Disable TLS cert verification:
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	var issues []Issue
	startAt := 0
	total := 1

	for startAt < total {
		cacheFilename := fmt.Sprintf("board.%d.issue.%d.json", boardId, startAt)

		jiraUrl := os.ExpandEnv("$JIRA_URL/rest/agile/1.0/board")
		url := fmt.Sprintf("%s/%d/issue?fields=changelog&expand=changelog&startAt=%d", jiraUrl, boardId, startAt)

		// Fetch from cache or network:
		issuesJsonBody, err := cachedGet(cacheFilename, url, cl)
		if err != nil {
			log.Fatal(err)
		}

		// Decode list of issues:
		pagedIssues := &PagedIssues{}
		dec := json.NewDecoder(issuesJsonBody)
		err = dec.Decode(pagedIssues)
		if err != nil {
			log.Fatal(err)
		}

		//fmt.Printf("%+v\n", pagedIssues)

		// Advance to next page:
		total = pagedIssues.Total
		startAt = pagedIssues.StartAt + len(pagedIssues.Issues)

		if issues == nil {
			issues = make([]Issue, 0, pagedIssues.Total)
		}

		// Append page:
		issues = append(issues, pagedIssues.Issues...)
	}

	now := time.Now()
	today := dateOf(now)

	// Discover latest status per issue:
	aging := make(map[string][]*Issue)
	for i := range issues {
		issue := &issues[i]

		issue.Status = ""
		issue.StatusTime = time.Unix(0, 0)

		for _, history := range issue.Changelog.Histories {
			for _, item := range history.Items {
				// Ignore any histories except status changes:
				if item.Field != "status" {
					continue
				}

				issue.Status = item.ToString
				issue.StatusTime = history.Created.Time
				issue.Assigned = history.Author
			}
		}

		if issue.Status == "" {
			continue
		}
		if issue.Status == "Open" || issue.Status == "Reopened" || issue.Status == "Closed" {
			continue
		}

		//statusList := aging[issue.Status]
		//if statusList == nil {
		//	statusList = make([]*Issue, 0, 10)
		//	aging[issue.Status] = statusList
		//}

		// subtract weekends:
		issue.StatusBusinessDays = dateOf(issue.StatusTime).BusinessDaysUntil(today)

		aging[issue.Status] = append(aging[issue.Status], issue)

		//fmt.Printf("%s: in %s since %v\n", issue.Key, issue.Status, issue.StatusTime)
	}

	for status, statusIssues := range aging {
		fmt.Printf("%s: [", status)
		for i, issue := range statusIssues {
			time.Now().Sub(issue.StatusTime)
			fmt.Printf("%s (%d)", issue.Key, issue.StatusBusinessDays)
			if i < len(statusIssues) - 1 {
				fmt.Print(", ")
			}
		}
		fmt.Printf("]\n")
	}
}
