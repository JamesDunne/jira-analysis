package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

func getEnvInt(key string, defaultValue int) int {
	tmpValue, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return defaultValue
	}

	return tmpValue
}

type zonedTimestamp struct {
	time.Time
}

func (t *zonedTimestamp) UnmarshalJSON(buf []byte) error {
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

//type IssueStatus struct {
//	Name string `json:"name"`
//}

type IssueFields struct {
	Summary string `json:"summary"`
	//Status   IssueStatus    `json:"status"`
	//Updated  zonedTimestamp `json:"updated"`
	//Assignee User           `json:"assignee"`

	// NOTE: this custom field name might vary by deployment?
	EpicName string `json:"customfield_12024"`
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

	cacheHit := false
	cacheAvailable := false
	if getEnvInt("JIRA_NOCACHE", 0) == 0 {
		stat, statErr := os.Stat(cacheFilename)

		cacheHit = statErr == nil || !os.IsNotExist(statErr)
		cacheAvailable = cacheHit
		if cacheHit && stat != nil {
			if stat.ModTime().Before(time.Now().Add(-time.Hour)) {
				cacheHit = false
			}
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
		req.SetBasicAuth(os.Getenv("JIRA_USERNAME"), os.Getenv("JIRA_PASSWORD"))

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

			httpErr := fmt.Errorf("HTTP response %s", rsp.Status)
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
	fmt.Println(`environment variables:
JIRA_URL      = base URL of JIRA website without trailing slash
JIRA_USERNAME = username to authenticate with
JIRA_PASSWORD = password to authenticate with

JIRA_BOARDID  = board ID to query status of
JIRA_JQL      = custom JQL filter to apply; default='status not in (closed, canceled, open, reopened, Analysis, "Analysis - 1")'
`)
	args := os.Args[1:]

	//boardId := 2924
	//boardId := 3612
	//boardId := 3581
	//boardId := 4085
	//boardId := 4454
	boardId := getEnvInt("JIRA_BOARDID", 4454)

	if len(args) >= 1 {
		intValue, err := strconv.Atoi(args[0])
		if err == nil {
			boardId = intValue
		}
	}

	if os.Getenv("JIRA_URL") == "" {
		os.Setenv("JIRA_URL", "https://ultidev")
	}

	jql := os.Getenv("JIRA_JQL")
	if jql == "" {
		jql = `status not in (closed, canceled, open, reopened, Analysis, "Analysis - 1")`
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
		url := fmt.Sprintf(
			"%s/%d/issue?expand=changelog&startAt=%d&jql=%s",
			jiraUrl,
			boardId,
			startAt,
			url.QueryEscape(jql),
		)

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
	for i := range issues {
		issue := &issues[i]

		if issue.Fields.EpicName != "" {
			continue
		}

		issue.StatusTime = time.Unix(0, 0)

		for _, history := range issue.Changelog.Histories {
			for _, item := range history.Items {
				// Ignore any fields except status changes:
				if item.Field != "status" {
					continue
				}

				issue.Status = item.ToString
				if item.ToString == "In Progress" {
					issue.StatusTime = history.Created.Time
				}
				issue.Assigned = history.Author
			}
		}

		if issue.Status == "" {
			continue
		}
		//if issue.Status == "Open" || issue.Status == "Reopened" || issue.Status == "Closed" {
		//	continue
		//}

		// Determine age in business days:
		issue.StatusBusinessDays = DateOf(issue.StatusTime).BusinessDaysUntil(today)

		// Add to status map:
		aging[issue.Status] = append(aging[issue.Status], issue)
	}

	//keys := []string{
	//	"In Progress",     // In Development
	//	"In Progress - 1", // PR
	//	"In Progress - 2", // Ready for QA
	//	"In Testing",      // In Testing
	//	//"Approved",
	//}
	names := map[string]string{
		"In Progress":     "In Development",
		"In Progress - 1": "PR",
		"In Progress - 2": "Ready for QA",
		"In Testing":      "In Testing",
	}

	keys := make([]string, 0, len(aging))
	for key := range aging {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	timeLayout := "Mon Jan 02"
	fmt.Printf("Now: %s\n", now.Format(timeLayout))
	for _, status := range keys {
		// sort issues by time descending:
		statusIssues := IssueList(aging[status])
		sort.Sort(statusIssues)

		friendlyName, ok := names[status]
		if ok {
			friendlyName = fmt.Sprintf(" (%s)", friendlyName)
		}
		fmt.Printf("%s%s: [\n", status, friendlyName)
		for _, issue := range statusIssues {
			//jb, _ := json.Marshal(issue)
			//fmt.Printf("%s\n", string(jb))

			time.Now().Sub(issue.StatusTime)
			fmt.Printf(
				"  %20s: %s (%2d days old since %s); %s\n",
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
