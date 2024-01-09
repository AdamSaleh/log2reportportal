package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bitfield/script"
	"github.com/go-resty/resty/v2"
	"golang.org/x/exp/maps"
)

type RPLaunch struct {
	Name      string      `json:"name,omitempty"`
	UUID      string      `json:"uuid,omitempty"`
	ID        json.Number `json:"id,omitempty"`
	RerunOf   string      `json:"rerunOf,omitempty"`
	StartTime int         `json:"startTime,omitempty"`
	EndTime   int         `json:"endTime,omitempty"`
	Rerun     bool        `json:"rerun,omitempty"`
}

type Launches struct {
	Content []RPLaunch `json:"content"`
}

type RPItem struct {
	Name        string      `json:"name,omitempty"`
	Type        string      `json:"type,omitempty"`
	LaunchUUID  string      `json:"launchUuid"`
	Description string      `json:"description"`
	UUID        string      `json:"uuid,omitempty"`
	ID          json.Number `json:"id,omitempty"`
	StartTime   int         `json:"startTime,omitempty"`
	EndTime     int         `json:"endTime,omitempty"`
}

/*func (i *RPItem) setUUID(uuid string) {
	i.UUID = uuid
}*/

type RPFinishItem struct {
	Type        string `json:"type"`
	LaunchUUID  string `json:"launchUuid"`
	Description string `json:"description"`
	Status      string `json:"status"`
	EndTime     int    `json:"endTime"`
}

type RPLog struct {
	LaunchUUID string `json:"launchUuid"`
	Time       string `json:"time"`
	ItemUUID   string `json:"itemUuid"`
	Message    string `json:"message"`
	Level      string `json:"level"`
	UUID       string `json:"uuid,omitempty"`
}

/*func (i *RPLog) setUUID(uuid string) {
	i.UUID = uuid
}*/

type ResultID struct {
	ID string `json:"id"` // beware, returns uuid
}

type RPLogger struct {
	project   string
	authToken string
	launch    *RPLaunch
	suite     *RPItem
	client    *resty.Client
	Tests     []*RPItem
}

func (p *RPLogger) requestWithAuth() *resty.Request {
	return p.client.R().SetAuthToken(p.authToken)
}

func (p *RPLogger) gPortalItem(apiPath, parent, id string, item interface{}) {
	url := fmt.Sprintf("/%s/%s", apiPath, id)
	if parent != "" {
		url = fmt.Sprintf("/%s/%s/%s", apiPath, parent, id)
	}
	_, err := p.requestWithAuth().
		SetResult(item).
		Get(url)
	if err != nil {
		panic(fmt.Errorf("Error:%w", err))
	}
}

func (p *RPLogger) uPortalItem(apiPath, parent, id string, item interface{}) {
	url := fmt.Sprintf("/%s/%s", apiPath, id)
	if parent != "" {
		url = fmt.Sprintf("/%s/%s/%s", apiPath, parent, id)
	}
	_, err := p.requestWithAuth().
		SetBody(item).
		Put(url)
	if err != nil {
		panic(fmt.Errorf("Error:%w", err))
	}
}

func (p *RPLogger) cPortalItem(apiPath, parent string, item interface{}) {
	url := apiPath
	if parent != "" {
		url = fmt.Sprintf("/%s/%s", apiPath, parent)
	}
	resultID := &ResultID{}
	_, err := p.requestWithAuth().
		SetBody(item).
		SetResult(&resultID).
		Post(url)
	if err != nil {
		panic(fmt.Errorf("Error:%w", err))
	}
	p.gPortalItem(apiPath, parent, resultID.ID, item)
}

func (p *RPLogger) cAsyncPortalItem(apiPath, parent string, item interface{}) string {
	url := apiPath
	if parent != "" {
		url = fmt.Sprintf("/%s/%s", apiPath, parent)
	}
	resultID := &ResultID{}
	_, err := p.requestWithAuth().
		SetBody(item).
		SetResult(&resultID).
		Post(url)
	if err != nil {
		panic(fmt.Errorf("Error:%w", err))
	}

	return resultID.ID
}

func NewRPLogger(client *resty.Client, token, project string) *RPLogger {
	return &RPLogger{project: project, client: client, authToken: token}
}

func (p *RPLogger) getLaunch(name string) int {
	if p.launch != nil {
		return 0
	}
	return -1
}

func (p *RPLogger) getSuite(name string) int {
	if p.suite != nil && p.suite.Name == name {
		return 0
	}
	return -1
}

func (p *RPLogger) getCase(name string) int {
	currentCase := len(p.Tests) - 1
	if currentCase < 0 {
		return -1
	}
	if p.Tests[currentCase].Name == name {
		return currentCase
	}
	for i, v := range p.Tests {
		if v.Name == name {
			return i
		}
	}
	return -1
}

func toUnix(startTime string) int {
	tt, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		panic(fmt.Errorf("Error:%w", err))
	}
	return int(tt.Unix()) * 1000 // api needs millisecond instead of seconds
}

func (p *RPLogger) EnsureLaunch(name, suite, startTime string) {
	if p.getLaunch(name) < 0 {
		l := &RPLaunch{Name: name, StartTime: toUnix(startTime), Rerun: false}
		p.cPortalItem(fmt.Sprintf("api/v1/%s/launch", p.project), "", l)
		p.launch = l
	}
	if p.launch.UUID == "" {
		p.gPortalItem(fmt.Sprintf("api/v1/%s/launch", p.project), "", string(p.launch.ID), p.launch)
	}
	if p.getSuite(suite) < 0 {
		s := &RPItem{Name: suite, Type: "suite", LaunchUUID: p.launch.UUID, StartTime: toUnix(startTime)}
		fmt.Println(s)
		p.cPortalItem(fmt.Sprintf("api/v1/%s/item", p.project), "", s)
		p.suite = s
	}
	if p.suite.UUID == "" {
		p.gPortalItem(fmt.Sprintf("api/v1/%s/item", p.project), "", string(p.suite.ID), p.suite)
	}
}

func (p *RPLogger) EnsureTest(name, startTime string) {
	if p.getCase(name) >= 0 {
		return
	}
	t := toUnix(startTime)
	uuid := p.launch.UUID
	ts := &RPItem{Name: name, StartTime: t, Type: "test", LaunchUUID: uuid, Description: name}
	ts.UUID = p.cAsyncPortalItem(fmt.Sprintf("api/v2/%s/item", p.project), p.suite.UUID, ts)
	p.Tests = append(p.Tests, ts)
}

func (p *RPLogger) AddLine(name, startTime, level, message string) {
	fmt.Printf("LOG: %s %s %s %s", name, startTime, level, message)
	p.EnsureTest(name, startTime)
	currentCase := p.getCase(name)
	fmt.Printf("LOG:CASE %v", currentCase)
	l := &RPLog{
		LaunchUUID: p.launch.UUID,
		ItemUUID:   p.Tests[currentCase].UUID,
		Time:       startTime,
		Message:    message,
		Level:      level,
	}
	fmt.Printf("LOG:CASE %v", l)
	p.cAsyncPortalItem(fmt.Sprintf("api/v2/%s/log/entry", p.project), "", l)
	l.ItemUUID = ""
	p.cAsyncPortalItem(fmt.Sprintf("api/v2/%s/log/entry", p.project), "", l)
}

func (p *RPLogger) FinnishTest(name, startTime, result, t string) {
	value, err := strconv.ParseFloat(t, 32)
	if err != nil {
		panic(fmt.Errorf("Error:%w", err))
	}

	p.EnsureTest(name, startTime)
	currentCase := p.getCase(name)

	ts := p.Tests[currentCase]
	f := &RPFinishItem{
		EndTime:    ts.StartTime + int(value)*1000,
		LaunchUUID: ts.LaunchUUID,
		Status:     "passed",
	}
	if result == "FAIL" {
		f.Status = "failed"
	}
	if result == "SKIP" {
		f.Status = "skipped"
	}
	p.uPortalItem(fmt.Sprintf("api/v1/%s/item", p.project), "", ts.UUID, f)
}

func (p *RPLogger) Finish(t string) {
	if p.suite != nil {
		p.uPortalItem(fmt.Sprintf("api/v1/%s/item", p.project), "", p.suite.UUID,
			&RPItem{EndTime: toUnix(t), LaunchUUID: p.launch.UUID})
	}
	p.uPortalItem(fmt.Sprintf("api/v1/%s/launch", p.project), p.launch.UUID, "finish",
		&RPItem{EndTime: toUnix(t)})
}

func getMatches(re *regexp.Regexp, str string) map[string]string {
	n1 := re.SubexpNames()
	r0 := re.FindStringSubmatch(str)
	m := map[string]string{}
	if len(r0) > 1 {
		for i, n := range r0[1:] {
			if val, ok := m[n1[i+1]]; !ok || val == "" {
				m[n1[i+1]] = n
			}
		}
	}
	return m
}

type Lines interface {
	reEND() string
	reSTAMP() string
	reRUN() string
	reLOG() string
	rePAUSE() string
	reCONT() string
}

type DefaultLines struct{}

func (l *DefaultLines) reCONT() string {
	return `^=== CONT\W*(?:kuttl/harness/)?(?P<test>[\w/\-_]*)/?(?P<step>[\w-_]*)?.*$`
}

func (l *DefaultLines) reEND() string {
	return `^.*--- (?P<result>\w+): (?:kuttl/harness/)?(?P<test>[\w/\-_]+)\W*\((?P<duration>\w+\.?\w*)s.*$`
}

func (l *DefaultLines) reSTAMP() string {
	return `^.*startTime.*"(?P<startDate>[0-9-:]*)T.*"`
}

func (l *DefaultLines) reRUN() string {
	return `^=== RUN\W*(?:kuttl/harness/)?(?P<test>[\w/\-_]*)/?(?P<step>[\w-_]*)?.*$`
}

func (l *DefaultLines) reLOG() string {
	argo := `time="(?P<date>[0-9-:]*)T(?P<timestamp>\d\d:\d\d:\d\d)Z".*level=(?P<level>\w+).*msg="(?P<msg>.*)".*`
	kuttl := `.*logger.*(?P<timestamp>\d\d:\d\d:\d\d) \| (?P<test>[\w-_]*)/?(?P<step>[\w-_]*)? \|(?P<msg>.*)`
	return fmt.Sprintf(`(?:^%s$|^%s$)`, argo, kuttl)
}

func (l *DefaultLines) rePAUSE() string {
	return `^=== PAUSE\W*(?:kuttl/harness/)?(?P<test>[\w/\-_]*)/?(?P<step>[\w-_]*)?.*$`
}

type PatternActions struct {
	pattern *regexp.Regexp
	actions []func(s, m map[string]string) map[string]string
}

type StateMachine struct {
	state            map[string]string
	patternToActions []*PatternActions
	noErrors         bool
}

func mkMachine(initialState map[string]string, noErrors bool) *StateMachine {
	return &StateMachine{state: initialState, patternToActions: []*PatternActions{}, noErrors: noErrors}
}

func (m *StateMachine) pattern(r string, a ...func(s, m map[string]string) map[string]string) *StateMachine {
	rx := regexp.MustCompile(r)

	m.patternToActions = append(m.patternToActions, &PatternActions{pattern: rx, actions: a})
	return m
}

func (m *StateMachine) feed(line string) {
	for _, pa := range m.patternToActions {
		if mt := getMatches(pa.pattern, line); len(mt) > 0 {
			for _, f := range pa.actions {
				func() {
					defer func() {
						if m.noErrors {
							if r := recover(); r != nil {
								fmt.Printf("Recovered in f - %v\n", r)
							}
						}
					}()
					m.state = f(m.state, mt)
				}()
			}
			return
		}
	}
}

func mapCopy(dst, src map[string]string) map[string]string {
	maps.Copy(dst, src)
	return dst
}

type TestReportBuilder interface {
	getLaunch(name string) int
	getCase(name string) int
	EnsureTest(name, startTime string)
	AddLine(name, startTime, level, message string)
	FinnishTest(name, startTime, result, time string)
	Finish(time string)
	EnsureLaunch(name, suite, startTime string)
}

func processLinear(lg TestReportBuilder, launchName, suiteName string, filePipe *script.Pipe, noErrors bool) {
	r := &DefaultLines{}
	m := mkMachine(map[string]string{"test": "", "level": "", "startDate": "", "time": "", "launch": ""},
		noErrors).
		pattern(r.reSTAMP(), mapCopy).
		pattern(r.reCONT(), mapCopy).
		pattern(r.rePAUSE(), mapCopy).
		pattern(r.reRUN(), mapCopy).
		pattern(r.reLOG(), mapCopy,
			func(s, m map[string]string) map[string]string {
				if s["test"] != "" {
					s["time"] = fmt.Sprintf("%s%sT%sZ", s["startDate"], m["date"], m["timestamp"])
					lg.EnsureLaunch(launchName, suiteName, s["time"])
					lg.AddLine(s["test"], s["time"], s["level"], m["msg"])
				}
				return s
			}).pattern(r.reEND(),
		mapCopy,
		func(s, m map[string]string) (o map[string]string) {
			if s["test"] != "" {
				lg.FinnishTest(s["test"], s["time"], m["result"], m["duration"])
			}
			return s
		},
	).pattern("(?P<line>^.*$)",
		func(s, m map[string]string) map[string]string {
			if s["test"] != "" && s["time"] != "" {
				lg.EnsureTest(s["test"], s["time"])
				lg.AddLine(s["test"], s["time"], s["level"], m["line"])
			}
			return s
		},
	)
	_, errPipe := filePipe.FilterLine(func(line string) string {
		m.feed(line)
		return line
	}).Stdout()
	if errPipe != nil {
		fmt.Println(errPipe)
	}
	lg.Finish(m.state["time"])
}

func firstLaunchIDWithName(client *resty.Client, token, portalURL, project, reportName string) string {
	url := fmt.Sprintf("%s/api/v1/%s/launch?filter.eq.name=%s", portalURL, project, reportName)
	fmt.Println(url)
	r, err := http.NewRequestWithContext(context.Background(), "GET", url, http.NoBody)
	if err != nil {
		panic(err)
	}
	r.Header.Add("Authorization", "Bearer "+token)
	c, err := script.NewPipe().WithHTTPClient(client.GetClient()).
		Do(r).JQ(".content[0].id").String()
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(c)
}

func firstSuiteIDWithName(client *resty.Client, token, portalURL, project, launchID, suiteName string) string {
	url := fmt.Sprintf("%s/api/v1/%s/item?filter.eq.launchId=%s&filter.eq.name=%s",
		portalURL, project, launchID, suiteName)
	fmt.Println(url)

	r, err := http.NewRequestWithContext(context.Background(), "GET", url, http.NoBody)
	if err != nil {
		panic(err)
	}
	r.Header.Add("Authorization", "Bearer "+token)
	c, err := script.NewPipe().WithHTTPClient(client.GetClient()).
		Do(r).JQ(".content[0].id").String()
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(c)
}

const nullResult = "null"

func run(portalURL, token, reportProject, reportName, suiteName, logFile string, skipTLS, skipExisting, noErrors bool) {
	client := resty.New()
	client.SetBaseURL(portalURL)
	client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: skipTLS})
	client.SetAuthToken(token)

	lg := NewRPLogger(client, token, reportProject)

	lid := firstLaunchIDWithName(client, token, portalURL, reportProject, reportName)
	if lid != nullResult {
		sid := firstSuiteIDWithName(client, token, portalURL, reportProject, lid, suiteName)

		if (sid != nullResult) && skipExisting {
			fmt.Printf("Suite %s in launch %s already reported\n", suiteName, reportName)
			return
		}

		// we are uploading new suite to existing launch, so we should pre-fill the launch
		lg.launch = &RPLaunch{Name: reportName, ID: json.Number(lid), UUID: ""}
		if sid != nullResult {
			lg.suite = &RPItem{Name: suiteName, ID: json.Number(sid), UUID: ""}
		}
	}

	if logFile == "-" {
		processLinear(lg, reportName, suiteName, script.Stdin(), noErrors)
	} else {
		processLinear(lg, reportName, suiteName, script.File(logFile), noErrors)
	}
}

func main() {
	var logFile string
	var suiteName string
	var reportName string
	var reportProject string
	var portalURL string
	var skipTLS bool
	var skipExisting bool
	var ignoreErrors bool

	t := time.Now()
	flag.StringVar(&logFile, "file", "", "path to the logfile, will assume stdin if set to -")
	flag.StringVar(&reportName, "launch", fmt.Sprintf("run%s", t.Format("20060102150405")), "name of the report")
	flag.StringVar(&suiteName, "name", fmt.Sprintf("run%s", t.Format("20060102150405")), "name of the report")
	flag.StringVar(&reportProject, "project", "gitops-adhoc", "project to upload to")
	flag.StringVar(&portalURL, "url",
		"https://reportportal-gitops-qe.apps.ocp-c1.prod.psi.redhat.com", "url of the report portal")
	flag.BoolVar(&skipTLS, "skipTls", false, "skip TLS checks")
	flag.BoolVar(&skipExisting, "skipExisting", false, "skip existing launches")
	flag.BoolVar(&ignoreErrors, "ignoreErrors", false, "recover from all panics")

	flag.Parse()

	token, ok := os.LookupEnv("RP_TOKEN")
	if !ok {
		panic("RP_TOKEN env var needs to be set to authenticate")
	}
	run(portalURL, token, reportProject, reportName, suiteName, logFile, skipTLS, skipExisting, ignoreErrors)
}
