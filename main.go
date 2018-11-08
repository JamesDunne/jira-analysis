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
	"sort"
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

type IssueFields struct {
	Summary string `json:"summary"`
}

type Issue struct {
	Id        string         `json:"id"`
	Key       string         `json:"key"`
	Fields    IssueFields    `json:"fields"`
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

func DateOf(t time.Time) Date {
	// Grab local date:
	//_, zoneOffset := t.Zone()
	l := t.Location()
	y, m, d := t.Date()
	// Build new date:
	return Date{time.Date(y, m, d, 6, 0, 0, 0, l)}
}

func (date Date) NextDate() Date {
	return DateOf(date.Time.Add(25 * time.Hour))
}

func (date Date) BusinessDaysUntil(until Date) int {
	// Count weekdays, skipping weekends:
	days := 0
	d := date

	_, startOffset := date.Zone()
	_, untilOffset := until.Zone()
	untilTime := until.In(date.Location()).Add(time.Duration(untilOffset-startOffset) * time.Second)
	//fmt.Printf("from %s to %s\n", date.Time, untilTime)

	for d.Time.Before(untilTime) {
		//fmt.Printf("  %d %s\n", days, d)

		days++
		d = d.NextDate()

		if d.Time.Weekday() == time.Saturday {
			d = d.NextDate()
		}
		if d.Time.Weekday() == time.Sunday {
			d = d.NextDate()
		}
	}

	//fmt.Printf("  %d %s\n", days, d)

	return days
}

type IssueList []*Issue

func (issues IssueList) Len() int {
	return len(issues)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (issues IssueList) Less(i, j int) bool {
	return issues[i].StatusBusinessDays > issues[j].StatusBusinessDays
}

// Swap swaps the elements with indexes i and j.
func (issues IssueList) Swap(i, j int) {
	issues[i], issues[j] = issues[j], issues[i]
}

func cachedGet(cacheFilename string, url string, cl *http.Client) (issuesJsonBody io.ReadCloser, err error) {
	log.Printf("GET '%s'\n", url)

	stat, statErr := os.Stat(cacheFilename)

	cacheHit := statErr == nil || !os.IsNotExist(statErr)
	cacheAvailable := cacheHit
	if cacheHit && stat != nil {
		if stat.ModTime().Before(time.Now().Add(-time.Hour)) {
			cacheHit = false
		}
	}

	respondCache := func() io.ReadCloser {
		if !cacheAvailable {
			return nil
		}

		b, err := ioutil.ReadFile(cacheFilename)
		if err != nil {
			return nil
		}

		log.Printf("cached response\n")
		issuesJsonBody = ioutil.NopCloser(bytes.NewReader(b))
		return issuesJsonBody
	}

	if cacheHit {
		issuesJsonBody = respondCache()
		if issuesJsonBody != nil {
			return issuesJsonBody, nil
		}
		cacheHit = false
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
			issuesJsonBody = respondCache()
			if issuesJsonBody != nil {
				return issuesJsonBody, nil
			}

			log.Printf("http: %v\n", err)
			return nil, err
		}
		defer rsp.Body.Close()

		if rsp.StatusCode >= 300 {
			log.Printf("http: status %s\n", rsp.Status)

			issuesJsonBody = respondCache()
			if issuesJsonBody != nil {
				return issuesJsonBody, nil
			}

			httpErr := errors.Errorf("HTTP response %s", rsp.Status)
			return nil, httpErr
		}

		var b []byte
		b, err = ioutil.ReadAll(rsp.Body)
		if err != nil {
			issuesJsonBody = respondCache()
			if issuesJsonBody != nil {
				return issuesJsonBody, nil
			}

			return nil, err
		}

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
		url := fmt.Sprintf("%s/%d/issue?expand=changelog&startAt=%d", jiraUrl, boardId, startAt)

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
	today := DateOf(now)

	// Discover latest status per issue:
	aging := make(map[string][]*Issue)
issue:
	for i := range issues {
		issue := &issues[i]

		issue.Status = ""
		issue.StatusTime = time.Unix(0, 0)

		for _, history := range issue.Changelog.Histories {
			for _, item := range history.Items {
				if item.Field == "Epic Name" {
					// Skip this issue since it is an epic:
					continue issue
				}
				// Ignore any fields except status changes:
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

		// Determine age in business days:
		issue.StatusBusinessDays = DateOf(issue.StatusTime).BusinessDaysUntil(today)

		// Add to status map:
		aging[issue.Status] = append(aging[issue.Status], issue)
	}

	keys := []string{
		"In Progress",     // In Development
		"In Progress - 1", // PR
		"In Progress - 2", // Ready for QA
		"In Testing",      // In Testing
		//"Approved",
	}
	names := map[string]string{
		"In Progress":     "In Development",
		"In Progress - 1": "PR",
		"In Progress - 2": "Ready for QA",
		"In Testing":      "In Testing",
	}

	timeLayout := "Mon Jan 02"
	fmt.Printf("Now: %s\n", now.Format(timeLayout))
	for _, status := range keys {
		statusIssues := IssueList(aging[status])
		sort.Sort(statusIssues)

		fmt.Printf("%s: [\n", names[status])
		for _, issue := range statusIssues {
			time.Now().Sub(issue.StatusTime)
			fmt.Printf(
				"  %12s: %s (%2d days old since %s); %s\n",
				issue.Assigned.UserName,
				issue.Key,
				issue.StatusBusinessDays,
				issue.StatusTime.Format(timeLayout),
				issue.Fields.Summary,
			)
		}
		fmt.Printf("]\n")
	}
}
