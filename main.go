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
	StartTime int    `json:"startTime,omitempty"`
	EndTime   int    `json:"endTime,omitempty"`
	Uuid      string `json:"uuid,omitempty"`
	Id        int    `json:"id,omitempty"`
}

func (i *RPLaunch) setUuid(uuid string) {
	i.Uuid = uuid
}

type Launches struct {
	Content []RPLaunch `json:"content"`
}

type RPWithUUID interface {
	setUuid(uuid string)
}

type RPItem struct {
	Name        string `json:"name,omitempty"`
	StartTime   int    `json:"startTime,omitempty"`
	Type        string `json:"type,omitempty"`
	LaunchUuid  string `json:"launchUuid"`
	EndTime     int    `json:"endTime,omitempty"`
	Description string `json:"description"`
	Uuid        string `json:"uuid,omitempty"`
	Id          int    `json:"id,omitempty"`
}

func (i *RPItem) setUuid(uuid string) {
	i.Uuid = uuid
}

type RPFinishItem struct {
	EndTime     int    `json:"endTime"`
	Type        string `json:"type"`
	LaunchUuid  string `json:"launchUuid"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type RPLog struct {
	LaunchUuid string `json:"launchUuid"`
	Time       string `json:"time"`
	ItemUuid   string `json:"itemUuid"`
	Message    string `json:"message"`
	Level      string `json:"level"`
	Uuid       string `json:"uuid,omitempty"`
}

func (i *RPLog) setUuid(uuid string) {
	i.Uuid = uuid
}

type ResultId struct {
	Id string `json:"id"`
}

type RPLogger struct {
	launch    *RPLaunch
	Tests     []*RPItem
	project   string
	client    *resty.Client
	authToken string
	url       string
}

func (rp *RPLogger) requestWithAuth() *resty.Request {
	return rp.client.R().SetAuthToken(rp.authToken)
}

func (rp *RPLogger) gPortalItem(apiPath string, id string, item interface{}) error {
	url := fmt.Sprintf("/%s/%s", apiPath, id)
	_, err := rp.requestWithAuth().
		SetResult(item).
		Get(url)
	if err != nil {
		panic(fmt.Errorf("Error:%w\n", err))
	}
	return nil
}
func (rp *RPLogger) uPortalItem(apiPath string, id string, item interface{}) error {

	url := fmt.Sprintf("/%s/%s", apiPath, id)
	_, err := rp.requestWithAuth().
		SetBody(item).
		Put(url)
	if err != nil {
		panic(fmt.Errorf("Error:%w\n", err))
	}
	return nil

}
func (rp *RPLogger) cPortalItem(apiPath string, item interface{}) error {
	resultId := &ResultId{}
	_, err := rp.requestWithAuth().
		SetBody(item).
		SetResult(&resultId).
		Post(apiPath)
	if err != nil {
		panic(fmt.Errorf("Error:%w\n", err))
	}

	rp.gPortalItem(apiPath, resultId.Id, item)
	return nil

}

func (rp *RPLogger) cAsyncPortalItem(apiPath string, item interface{}) string {
	resultId := &ResultId{}
	_, err := rp.requestWithAuth().
		SetBody(item).
		SetResult(&resultId).
		Post(apiPath)
	if err != nil {
		panic(fmt.Errorf("Error:%w\n", err))
	}

	return resultId.Id
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
		panic(fmt.Errorf("Error:%w\n", err))
	}
	return int(tt.Unix()) * 1000
}

func (p *RPLogger) Launch(name, startTime string) {
	l := &RPLaunch{Name: name, StartTime: toUnix(startTime)}
	p.cPortalItem(fmt.Sprintf("api/v1/%s/launch", p.project), l)
	p.launch = l
}

func (p *RPLogger) StartTest(name, startTime string) {
	t := toUnix(startTime)
	uuid := p.launch.Uuid
	ts := &RPItem{Name: name, StartTime: t, Type: "test", LaunchUuid: uuid, Description: name}
	ts.Uuid = p.cAsyncPortalItem(fmt.Sprintf("api/v2/%s/item", p.project), ts)
	p.Tests = append(p.Tests, ts)
}
func (p *RPLogger) AddLine(name, startTime, level, message string) {
	fmt.Printf("LOG: %s %s %s %s", name, startTime, level, message)
	currentCase := p.getCase(name)
	fmt.Printf("LOG:CASE %v", currentCase)
	l := &RPLog{
		LaunchUuid: p.launch.Uuid,
		ItemUuid:   p.Tests[currentCase].Uuid,
		Time:       startTime,
		Message:    message,
		Level:      level,
	}
	fmt.Printf("LOG:CASE %v", l)
	p.cAsyncPortalItem(fmt.Sprintf("api/v2/%s/log/entry", p.project), l)

}

func (p *RPLogger) FinnishTest(name, result, time string) {
	fmt.Printf("Finishing:%s %v\n", name, result)
	value, err := strconv.ParseFloat(time, 32)
	if err != nil {
		panic(fmt.Errorf("Error:%w\n", err))
	}
	currentCase := p.getCase(name)
	if currentCase < 0 {
		return
	}
	ts := p.Tests[currentCase]
	f := &RPFinishItem{
		EndTime:    ts.StartTime + int(value)*1000,
		LaunchUuid: ts.LaunchUuid,
		Status:     "passed",
	}
	if result == "FAIL" {
		f.Status = "failed"
	}
	if result == "SKIP" {
		f.Status = "skipped"
	}
	p.uPortalItem(fmt.Sprintf("api/v1/%s/item", p.project), ts.Uuid, f)
}
func (p *RPLogger) Finish(time string) {
	p.uPortalItem(fmt.Sprintf("api/v1/%s/launch/%s/finish", p.project, p.launch.Uuid), "", &RPItem{EndTime: toUnix(time)})
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
	return `(?:^time="(?P<date>[0-9-:]*)T(?P<timestamp>\d\d:\d\d:\d\d)Z".*level=(?P<level>\w+).*msg="(?P<msg>.*)".*$|^.*logger.*(?P<timestamp>\d\d:\d\d:\d\d) \| (?P<test>[\w-_]*)/?(?P<step>[\w-_]*)? \|(?P<msg>.*)$)`
}

func (l *DefaultLines) rePAUSE() string {
	return `^=== PAUSE\W*(?:kuttl/harness/)?(?P<test>[\w/\-_]*)/?(?P<step>[\w-_]*)?.*$`
}

type PatternActions struct {
	pattern *regexp.Regexp
	actions []func(s map[string]string, m map[string]string) map[string]string
}

type StateMachine struct {
	state            map[string]string
	patternToActions []*PatternActions
}

func mkMachine(initialState map[string]string) *StateMachine {
	return &StateMachine{state: initialState, patternToActions: []*PatternActions{}}
}

func (m *StateMachine) pattern(r string, a ...func(s map[string]string, m map[string]string) map[string]string) *StateMachine {
	rx := regexp.MustCompile(r)

	m.patternToActions = append(m.patternToActions, &PatternActions{pattern: rx, actions: a})
	return m
}

func (m *StateMachine) feed(line string) {
	for _, pa := range m.patternToActions {
		if mt := getMatches(pa.pattern, line); len(mt) > 0 {
			for _, f := range pa.actions {
				m.state = f(m.state, mt)
			}
			return
		}
	}
}

func m_cp(dst map[string]string, src map[string]string) map[string]string {
	//fmt.Printf("%v %v \n", dst, src)
	maps.Copy(dst, src)
	return dst
}

type TestReportBuilder interface {
	getLaunch(name string) int
	getCase(name string) int
	StartTest(name, startTime string)
	AddLine(name, startTime, level, message string)
	FinnishTest(name, result, time string)
	Finish(time string)
	Launch(name, startTime string)
}

func processLinear(lg TestReportBuilder, lname, lproject string, filePipe *script.Pipe) {
	r := &DefaultLines{}
	i := 0
	m := mkMachine(map[string]string{
		"test":      "",
		"level":     "",
		"startDate": "",
		"time":      "",
		"launch":    "",
	}).
		pattern(r.reSTAMP(), m_cp).
		pattern(r.reCONT(), m_cp).
		pattern(r.reRUN(), m_cp).
		pattern(r.reLOG(),
			m_cp,
			func(s map[string]string, m map[string]string) map[string]string {
				if s["test"] == "" {
					return s
				}
				s["time"] = fmt.Sprintf("%s%sT%sZ", s["startDate"], m["date"], m["timestamp"])
				if lg.getLaunch(lname) < 0 {
					lg.Launch(lname, s["time"])
				}
				if lg.getCase(s["test"]) < 0 {
					lg.StartTest(s["test"], s["time"])
				}
				lg.AddLine(s["test"], s["time"], s["level"], m["msg"])
				return s
			}).pattern(r.reEND(),
		m_cp,
		func(s map[string]string, m map[string]string) map[string]string {
			if s["test"] == "" {
				return s
			}
			if lg.getCase(s["test"]) < 0 {
				lg.StartTest(s["test"], s["time"])
			}
			lg.FinnishTest(s["test"], m["result"], m["duration"])
			return s
		},
	).pattern("(?P<line>^.*$)",
		func(s map[string]string, m map[string]string) map[string]string {
			if (s["test"] != "" && s["time"] != "") && s["level"] != "" {
				if lg.getCase(s["test"]) < 0 {
					lg.StartTest(s["test"], s["time"])
				}
				lg.AddLine(s["test"], s["time"], s["level"], m["line"])
			}
			return s
		},
	)
	filePipe.FilterLine(func(line string) string {
		i++
		//fmt.Printf("Processing line %v\n", i)
		m.feed(line)
		return line
	}).String()
	lg.Finish(m.state["time"])

}

var reportName string
var reportProject string
var portalUrl string

func main() {
	t := time.Now()
	flag.StringVar(&reportName, "name", fmt.Sprintf("run%s", t.Format("20060102150405")), "name of the report")
	flag.StringVar(&reportProject, "project", "gitops-adhoc", "project to upload to")
	flag.StringVar(&portalUrl, "url", "https://reportportal-gitops-qe.apps.ocp-c1.prod.psi.redhat.com", "url of the report portal")

	flag.Parse()

	token, ok := os.LookupEnv("RP_TOKEN")
	if !ok {
		panic("RP_TOKEN env var needs to be set to authenticate")
	}
	client := resty.New()
	client.SetBaseURL(portalUrl)
	client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	lg := NewRPLogger(client, token, reportProject)

	processLinear(lg, reportName, reportProject, script.Stdin())
}
