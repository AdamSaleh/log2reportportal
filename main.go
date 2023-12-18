package main

import (
	"crypto/tls"
	"flag"
	"fmt"

	"io"
	"regexp"
	"strconv"
	"time"

	"github.com/bitfield/script"
	"github.com/go-resty/resty/v2"
	"golang.org/x/exp/maps"
)

type RPLaunch struct {
	name      string `json:"name,omitempty"`
	startTime int    `json:"startTime,omitempty"`
	endTime   int    `json:"endTime,omitempty"`
	uuid      string `json:"uuid,omitempty"`
	id        int    `json:"id,omitempty"`
}

func (i *RPLaunch) setUuid(uuid string) {
	i.uuid = uuid
}

type Launches struct {
	content []RPLaunch `json:"content"`
}

type RPWithUUID interface {
	setUuid(uuid string)
}

type RPItem struct {
	name        string `json:"name,omitempty"`
	startTime   int    `json:"startTime,omitempty"`
	Type        string `json:"type,omitempty"`
	launchUuid  string `json:"launchUuid"`
	endTime     int    `json:"endTime,omitempty"`
	description string `json:"description"`
	uuid        string `json:"uuid,omitempty"`
	id          int    `json:"id,omitempty"`
}

func (i *RPItem) setUuid(uuid string) {
	i.uuid = uuid
}

type RPFinishItem struct {
	endTime     int    `json:"endTime"`
	Type        string `json:"type"`
	launchUuid  string `json:"launchUuid"`
	description string `json:"description"`
	status      string `json:"status"`
}

type RPLog struct {
	launchUuid string `json:"launchUuid"`
	time       string `json:"time"`
	itemUuid   string `json:"itemUuid"`
	message    string `json:"message"`
	level      string `json:"level"`
	uuid       string `json:"uuid,omitempty"`
}

func (i *RPLog) setUuid(uuid string) {
	i.uuid = uuid
}

type ResultId struct {
	id string `json:"id"`
}

func gPortalItem(client *resty.Client, apiPath string, id string, item interface{}) error {
	url := fmt.Sprintf("https://reportportal-gitops-qe.apps.ocp-c1.prod.psi.redhat.com/%s/%s", apiPath, id)
	_, err := client.R().
		SetAuthToken("").
		SetResult(item).
		Get(url)
	if err != nil {
		panic(fmt.Errorf("Error:%w\n", err))
	}
	return nil
}
func uPortalItem(client *resty.Client, apiPath string, id string, item interface{}) error {

	url := fmt.Sprintf("https://reportportal-gitops-qe.apps.ocp-c1.prod.psi.redhat.com/%s/%s", apiPath, id)
	_, err := client.R().
		SetAuthToken("").
		SetBody(item).
		Put(url)
	if err != nil {
		panic(fmt.Errorf("Error:%w\n", err))
	}
	return nil

}
func cPortalItem(client *resty.Client, apiPath string, item interface{}) error {
	url := fmt.Sprintf("https://reportportal-gitops-qe.apps.ocp-c1.prod.psi.redhat.com/%s", apiPath)
	resultId := &ResultId{}
	_, err := client.R().
		SetAuthToken("").
		SetBody(item).
		SetResult(&resultId).
		Post(url)
	if err != nil {
		panic(fmt.Errorf("Error:%w\n", err))
	}

	gPortalItem(client, apiPath, resultId.id, item)
	return nil

}

func cAsyncPortalItem(client *resty.Client, apiPath string, item interface{}) string {
	url := fmt.Sprintf("https://reportportal-gitops-qe.apps.ocp-c1.prod.psi.redhat.com/%s", apiPath)
	fmt.Printf("POST %v \n %+v \n\n", url, item)
	resultId := &ResultId{}
	_, err := client.R().
		SetAuthToken("").
		SetBody(item).
		SetResult(&resultId).
		Post(url)
	if err != nil {
		panic(fmt.Errorf("Error:%w\n", err))
	}

	return resultId.id
}

type RPLogger struct {
	launch  *RPLaunch
	Tests   []*RPItem
	project string
	client  *resty.Client
}

func NewRPLogger(client *resty.Client, project string) *RPLogger {
	return &RPLogger{project: project, client: client}
}

func (p *RPLogger) getCase(name string) int {
	currentCase := len(p.Tests) - 1
	if currentCase < 0 {
		return -1
	}
	if p.Tests[currentCase].name == name {
		return currentCase
	}
	for i, v := range p.Tests {
		if v.name == name {
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
	l := &RPLaunch{name: name, startTime: toUnix(startTime)}
	cPortalItem(p.client, fmt.Sprintf("api/v1/%s/launch", p.project), l)
	p.launch = l
}

func (p *RPLogger) StartTest(name, startTime string) {
	t := toUnix(startTime)
	uuid := p.launch.uuid
	ts := &RPItem{name: name, startTime: t, Type: "test", launchUuid: uuid, description: name}
	ts.uuid = cAsyncPortalItem(p.client, fmt.Sprintf("api/v2/%s/item", p.project), ts)
	p.Tests = append(p.Tests, ts)
}
func (p *RPLogger) AddLine(name, startTime, level, message string) {
	fmt.Printf("LOG: %s %s %s %s", name, startTime, level, message)
	currentCase := p.getCase(name)
	fmt.Printf("LOG:CASE %v", currentCase)
	l := &RPLog{
		launchUuid: p.launch.uuid,
		itemUuid:   p.Tests[currentCase].uuid,
		time:       startTime,
		message:    message,
		level:      level,
	}
	fmt.Printf("LOG:CASE %v", l)
	cAsyncPortalItem(p.client, fmt.Sprintf("api/v2/%s/log/entry", p.project), l)

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
		endTime:    ts.startTime + int(value)*1000,
		launchUuid: ts.launchUuid,
		status:     "passed",
	}
	if result == "FAIL" {
		f.status = "failed"
	}
	if result == "SKIP" {
		f.status = "skipped"
	}
	uPortalItem(p.client, fmt.Sprintf("api/v1/%s/item", p.project), ts.uuid, f)
}
func (p *RPLogger) Finish(time string) {
	uPortalItem(p.client, fmt.Sprintf("api/v1/%s/launch/%s/finish", p.project, p.launch.uuid), "", &RPItem{endTime: toUnix(time)})
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
	return `^=== CONT\W*(?:kuttl/harness/)?(?P<currentTest>[\w/\-_]*)/?(?P<step>[\w-_]*)?.*$`
}

func (l *DefaultLines) reEND() string {
	return `^.*--- (?P<result>\w+): (?:kuttl/harness/)?(?P<currentTest>[\w/\-_]+)\W*\((?P<duration>\w+\.?\w*)s.*$`
}

func (l *DefaultLines) reSTAMP() string {
	return `^.*startTime.*"(?P<startDate>[0-9-:]*)T.*"`
}

func (l *DefaultLines) reRUN() string {
	return `^=== RUN\W*(?:kuttl/harness/)?(?P<currentTest>[\w/\-_]*)/?(?P<step>[\w-_]*)?.*$`
}

func (l *DefaultLines) reLOG() string {
	return `(?:^time="(?P<date>[0-9-:]*)T(?P<timestamp>\d\d:\d\d:\d\d)Z".*level=(?P<level>\w+).*msg="(?P<msg>.*)".*$|^.*logger.*(?P<timestamp>\d\d:\d\d:\d\d) \| (?P<currentTest>[\w-_]*)/?(?P<step>[\w-_]*)? \|(?P<msg>.*)$)`
}

func (l *DefaultLines) rePAUSE() string {
	return `^=== PAUSE\W*(?:kuttl/harness/)?(?P<currentTest>[\w/\-_]*)/?(?P<step>[\w-_]*)?.*$`
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
	maps.Copy(dst, src)
	return dst
}

func uploadLinear(lname, lproject string, filePipe *script.Pipe) {
	r := &DefaultLines{}
	client := resty.New()
	client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	lg := NewRPLogger(client, lproject)
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
				s["time"] = fmt.Sprintf("%s%sT%sZ", s["startDate"], m["date"], m["timestamp"])
				if lg.launch == nil {
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
			if lg.getCase(s["test"]) < 0 {
				lg.StartTest(s["test"], s["time"])
			}
			lg.FinnishTest(s["test"], m["result"], m["suration"])
			return s
		},
	).pattern("(?P<line>^.*$)",
		func(s map[string]string, m map[string]string) map[string]string {
			if lg.getCase(s["test"]) < 0 {
				lg.StartTest(s["test"], s["time"])
			}
			lg.AddLine(s["test"], s["time"], s["level"], m["line"])
			return s
		},
	)
	filePipe.FilterScan(func(line string, w io.Writer) {
		i++
		fmt.Printf("Processing line %v\n", i)
		m.feed(line)
	})

}

var reportName string
var reportProject string

func main() {
	t := time.Now()
	flag.StringVar(&reportName, "name", fmt.Sprintf("run%s", t.Format("20060102150405")), "name of the report")
	flag.StringVar(&reportProject, "project", "gitops-adhoc", "project to upload to")
	flag.Parse()
	uploadLinear(reportName, reportProject, script.Stdin())
}
