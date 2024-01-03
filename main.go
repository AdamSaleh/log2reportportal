package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/bitfield/script"
	"github.com/go-resty/resty/v2"
	"golang.org/x/exp/maps"
)

type RPLaunch struct {
	Name      string `json:"name,omitempty"`
	UUID      string `json:"uuid,omitempty"`
	StartTime int    `json:"startTime,omitempty"`
	EndTime   int    `json:"endTime,omitempty"`
	ID        int    `json:"id,omitempty"`
}

/*func (i *RPLaunch) setUUID(uuid string) {
	i.UUID = uuid
}*/

type Launches struct {
	Content []RPLaunch `json:"content"`
}

/*type RPWithUUID interface {
	setUUID(uuid string)
}*/

type RPItem struct {
	Name        string `json:"name,omitempty"`
	Type        string `json:"type,omitempty"`
	LaunchUUID  string `json:"launchUuid"`
	Description string `json:"description"`
	UUID        string `json:"uuid,omitempty"`
	StartTime   int    `json:"startTime,omitempty"`
	EndTime     int    `json:"endTime,omitempty"`
	ID          int    `json:"id,omitempty"`
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
	ID string `json:"id"`
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
		l := &RPLaunch{Name: name, StartTime: toUnix(startTime)}
		p.cPortalItem(fmt.Sprintf("api/v1/%s/launch", p.project), "", l)
		p.launch = l
	}
	if p.getSuite(suite) < 0 {
		s := &RPItem{Name: suite, Type: "suite", LaunchUUID: p.launch.UUID, StartTime: toUnix(startTime)}
		p.cPortalItem(fmt.Sprintf("api/v1/%s/item", p.project), "", s)
		p.suite = s
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
	p.uPortalItem(fmt.Sprintf("api/v1/%s/launch", p.project), p.launch.UUID, "finish", &RPItem{EndTime: toUnix(t)})
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
}

func mkMachine(initialState map[string]string) *StateMachine {
	return &StateMachine{state: initialState, patternToActions: []*PatternActions{}}
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
						if r := recover(); r != nil {
							fmt.Println("Recovered in f", r)
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

func processLinear(lg TestReportBuilder, launchName, suiteName string, filePipe *script.Pipe) {
	r := &DefaultLines{}
	m := mkMachine(map[string]string{"test": "", "level": "", "startDate": "", "time": "", "launch": ""}).
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

func main() {
	var suiteName string
	var reportName string
	var reportProject string
	var portalURL string
	var skipTLS bool

	t := time.Now()
	flag.StringVar(&reportName, "launch", fmt.Sprintf("run%s", t.Format("20060102150405")), "name of the report")
	flag.StringVar(&suiteName, "name", fmt.Sprintf("run%s", t.Format("20060102150405")), "name of the report")
	flag.StringVar(&reportProject, "project", "gitops-adhoc", "project to upload to")
	flag.StringVar(&portalURL, "url",
		"https://reportportal-gitops-qe.apps.ocp-c1.prod.psi.redhat.com", "url of the report portal")
	flag.BoolVar(&skipTLS, "skipTls", false, "skip TLS checks")

	flag.Parse()

	token, ok := os.LookupEnv("RP_TOKEN")
	if !ok {
		panic("RP_TOKEN env var needs to be set to authenticate")
	}
	client := resty.New()
	client.SetBaseURL(portalURL)
	client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: skipTLS})
	lg := NewRPLogger(client, token, reportProject)

	processLinear(lg, reportName, suiteName, script.Stdin())
}
