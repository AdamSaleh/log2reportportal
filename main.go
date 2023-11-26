package main

import (
	"encoding/xml"
	"fmt"
	"os"

	"crypto/tls"
	"io"
	"regexp"
	"strconv"
	"time"

	"github.com/bitfield/script"
	"github.com/go-resty/resty/v2"

	"github.com/AdamSaleh/log2reportportal/src"
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

func gPortalItem(apiPath string, id string, item interface{}) error {
	url := fmt.Sprintf("https://reportportal-gitops-qe.apps.ocp-c1.prod.psi.redhat.com/%s/%s", apiPath, id)
	r, err := client.R().
		SetAuthToken("").
		SetResult(item).
		Get(url)
	if err != nil {
		panic(fmt.Errorf("Error:%w\n", err))
	}

}
func uPortalItem(apiPath string, id string, item interface{}) error {
	client := resty.New()
	client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})

	url := fmt.Sprintf("https://reportportal-gitops-qe.apps.ocp-c1.prod.psi.redhat.com/%s/%s", apiPath, id)
	r, err := client.R().
		SetAuthToken("").
		SetBody(item).
		Put(url)
	if err != nil {
		panic(fmt.Errorf("Error:%w\n", err))
	}

}
func cPortalItem(apiPath string, item interface{}) error {
	client := resty.New()
	client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})

	url := fmt.Sprintf("https://reportportal-gitops-qe.apps.ocp-c1.prod.psi.redhat.com/%s", apiPath)
	resultId := &ResultId{}
	r, err := client.R().
		SetAuthToken("").
		SetBody(item).
		SetResult(&resultId).
		Post(url)
	if err != nil {
		panic(fmt.Errorf("Error:%w\n", err))
	}

	gPortalItem(apiPath, resultId.id, item)
}
func cAsyncPortalItem(apiPath string, item interface{}) string {
	client := resty.New()
	client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})

	url := fmt.Sprintf("https://reportportal-gitops-qe.apps.ocp-c1.prod.psi.redhat.com/%s", apiPath)
	fmt.Printf("POST %v \n %+v \n\n", url, item)
	resultId := &ResultId{}
	r, err := client.R().
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
}

func NewRPLogger(project string) *RPLogger {
	return &RPLogger{project: project}
}

func (p *RPLogger) getCase(name string) int {
	currentCase := len(p.Tests) - 1
	//fmt.Println("TESTS:")
	//fmt.Println(p.Tests)
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
	return int(tt.Unix()) * 1000
}

func (p *RPLogger) Launch(name, startTime string) {
	l := &RPLaunch{name: name, startTime: toUnix(startTime)}
	cPortalItem(fmt.Sprintf("api/v1/%s/launch", p.project), l)
	p.launch = l
}

func (p *RPLogger) StartTest(name, startTime string) {
	t := toUnix(startTime)
	uuid := p.launch.uuid
	ts := &RPItem{name: name, startTime: t, Type: "test", launchUuid: uuid, description: name}
	ts.uuid = cAsyncPortalItem(fmt.Sprintf("api/v2/%s/item", p.project), ts)
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
		launchUuid: p.launch.uuid,
		message:    message,
		level:      level,
	}
	fmt.Printf("LOG:CASE %v", l)
	cAsyncPortalItem(fmt.Sprintf("api/v2/%s/log/entry", p.project), l)

}

func (p *RPLogger) FinnishTest(name, result, time string) {
	fmt.Printf("Finishing:%s %v\n", name, result)
	value, err := strconv.ParseFloat(time, 32)

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
	uPortalItem(fmt.Sprintf("api/v1/%s/item", p.project), ts.uuid, f)
}
func (p *RPLogger) Finish(time string) {
	uPortalItem(fmt.Sprintf("api/v1/%s/launch/%s/finish", p.project, p.launch.uuid), "", &RPItem{endTime: toUnix(time)})
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
func uploadLinear(lname, logfile string) {
	reEND := regexp.MustCompile(`^.*--- (?P<result>\w+): (?:kuttl/harness/)?(?P<name>[\w/\-_]+)\W*\((?P<time>\w+\.?\w*)s.*$`)

	reSTAMP := regexp.MustCompile(`^.*startTime.*"(?P<date>[0-9-:]*)T.*"`)
	reRUN := regexp.MustCompile(`^=== RUN\W*(?:kuttl/harness/)?(?P<name>[\w/\-_]*)/?(?P<step>[\w-_]*)?.*$`)
	reLOG := regexp.MustCompile(`(?:^time="(?P<date>[0-9-:]*)T(?P<timestamp>\d\d:\d\d:\d\d)Z".*level=(?P<level>\w+).*msg="(?P<msg>.*)".*$|^.*logger.*(?P<timestamp>\d\d:\d\d:\d\d) \| (?P<name>[\w-_]*)/?(?P<step>[\w-_]*)? \|(?P<msg>.*)$)`)
	reCONT := regexp.MustCompile(`^=== CONT\W*(?:kuttl/harness/)?(?P<name>[\w/\-_]*)/?(?P<step>[\w-_]*)?.*$`)
	rePAUSE := regexp.MustCompile(`^=== PAUSE\W*(?:kuttl/harness/)?(?P<name>[\w/\-_]*)/?(?P<step>[\w-_]*)?.*$`)
	currentTest := ""
	started := false
	curentLevel := "info"
	startDate := ""
	currentTime := ""
	launched := true
	lg := NewRPLogger("gitops-adhoc")
	i := 0
	script.File(logfile).FilterScan(func(line string, w io.Writer) {
		i++
		//if i%100==0 {
		fmt.Printf("Processing line %v\n", i)
		//}

		if m := getMatches(reSTAMP, line); len(m) > 0 {
			// fmt.Printf("stamp %v\n",m)
			startDate = m["date"]
		} else if m := getMatches(reCONT, line); len(m) > 0 {
			//      fmt.Printf("cont %v\n",m)
			currentTest = m["name"]
		} else if m := getMatches(reRUN, line); len(m) > 0 {
			//  fmt.Printf("run %v\n",m)
			currentTest = m["name"]
			started = true
		} else if m := getMatches(reLOG, line); len(m) > 0 {
			// fmt.Printf("log %v\n",m)
			currentTime = fmt.Sprintf("%s%sT%sZ", startDate, m["date"], m["timestamp"])
			if val, ok := m["name"]; ok && val != "" {
				currentTest = m["name"]
			}
			if currentTest == "EMPTY" || currentTest == "" {
				return
			}
			if launched {
				lg.Launch(lname, currentTime)
				launched = false
			}
			if started || (lg.getCase(currentTest) < 0) {
				lg.StartTest(currentTest, currentTime)
				started = false
			}
			//fmt.Printf("%v, %s %s %s %v",lg,lname, currentTest, currentTime, m )
			if val, ok := m["level"]; ok && val != "" {
				curentLevel = m["level"]
			}
			lg.AddLine(currentTest, currentTime, curentLevel, m["msg"])
		} else if m := getMatches(reEND, line); len(m) > 0 {
			// fmt.Printf("end %v\n",m)
			// fmt.Printf("ENDING ON %s", line)
			if val, ok := m["name"]; ok && val != "" {
				currentTest = m["name"]
			}
			if started || (lg.getCase(currentTest) < 0) {
				lg.StartTest(currentTest, currentTime)
				started = false
			}
			e := m["result"]

			lg.FinnishTest(currentTest, e, m["time"])
		} else {
			if currentTest != "" && (lg.getCase(currentTest) > -1) && currentTime != "" && curentLevel != "" {
				lg.AddLine(currentTest, currentTime, curentLevel, line)
			}
		}
	}).Stdout()
	lg.Finish(currentTime)
}

type TCLog struct {
	XMLName xml.Name `xml:"system-out"`
	Log     string   `xml:",cdata"`
}
type TCFail struct {
	XMLName xml.Name `xml:"failure"`
	Message string   `xml:"message,attr"`
	Log     string   `xml:",cdata"`
}
type TCSkip struct {
	XMLName xml.Name `xml:"skipped"`
	Message string   `xml:"message,attr"`
	Log     string   `xml:",cdata"`
}

type TC struct {
	XMLName   xml.Name `xml:"testcase"`
	Name      string   `xml:"name,attr"`
	ClassName string   `xml:"classname,attr"`
	Time      string   `xml:"time,attr"`
	Log       *TCLog   `xml:",omitempty"`
	Failure   *TCFail  `xml:",omitempty"`
	Skip      *TCSkip  `xml:",omitempty"`
}

type TS struct {
	XMLName   xml.Name `xml:"testsuite"`
	Name      string   `xml:"name,attr"`
	Tests     int      `xml:"tests,attr"`
	Errors    int      `xml:"errors,attr"`
	Failures  int      `xml:"failures,attr"`
	Skipped   int      `xml:"skipped,attr"`
	Timestamp string   `xml:"name,attr"`
	Cases     []*TC
}

type XmlLogger struct {
	XMLName  xml.Name `xml:"testsuites"`
	Tests    int      `xml:"tests,attr"`
	Errors   int      `xml:"errors,attr"`
	Failures int      `xml:"failures,attr"`
	Skipped  int      `xml:"skipped,attr"`
	Suites   []*TS
}

func NewXmlLogger() *XmlLogger {
	return &XmlLogger{Tests: 0, Errors: 0, Failures: 0, Skipped: 0}
}

func (p *XmlLogger) Launch(name, startTime string) {
	ts := &TS{Name: name}
	p.Suites = append(p.Suites, ts)
}
func (p *XmlLogger) StartTest(name, startTime) {
	fmt.Printf("Started: %s\n", name)
	currentSuite := len(p.Suites) - 1
	if currentSuite < 0 {
		return
	}
	tc := &TC{Name: name, Log: &TCLog{}}
	p.Suites[currentSuite].Cases = append(p.Suites[currentSuite].Cases, tc)
}
func (p *XmlLogger) AddLine(startTime, level, message string) {
	currentSuite := len(p.Suites) - 1
	if currentSuite < 0 {
		return
	}
	currentCase := len(p.Suites[currentSuite].Cases) - 1
	if currentCase < 0 {
		return
	}
	p.Suites[currentSuite].Cases[currentCase].Log.Log += fmt.Sprintf(`time="%s" level=%s msg="%s"`, startTime, level, message) + "\n"
}
func (p *XmlLogger) FinnishTest(result, time string) {
	currentSuite := len(p.Suites) - 1
	if currentSuite < 0 {
		return
	}
	currentCase := len(p.Suites[currentSuite].Cases) - 1
	if currentCase < 0 {
		return
	}

	p.Tests += 1
	p.Suites[currentSuite].Tests += 1
	fmt.Printf("Finished: %s\n", p.Suites[currentSuite].Cases[currentCase].Name)
	if result == "PASS" {
		return
	}
	p.Suites[currentSuite].Cases[currentCase].Time = time
	log := p.Suites[currentSuite].Cases[currentCase].Log.Log
	p.Suites[currentSuite].Cases[currentCase].Log = nil

	if result == "SKIP" {
		p.Skipped += 1
		p.Suites[currentSuite].Skipped += 1
		p.Suites[currentSuite].Cases[currentCase].Skip = &TCSkip{Log: log, Message: "Skipped"}
		return
	}

	if result == "FAIL" {
		p.Failures += 1
		p.Suites[currentSuite].Failures += 1
		p.Suites[currentSuite].Cases[currentCase].Failure = &TCFail{Log: log, Message: "Failed"}

	}
}
func (p *XmlLogger) Finish() {
}

func processLinear() {

	reRUN := regexp.MustCompile(`^=== RUN\W*(?P<name>\w*).*$`)
	reLOG := regexp.MustCompile(`^time="(?P<timestamp>[0-9-:TZ]*)".*level=(?P<level>\w+).*msg="(?P<msg>.*)".*$`)
	reEND := regexp.MustCompile(`^--- (?P<result>\w+): (?P<name>\w+) .*\((?time\w)\)s.*$`)

	currentTest := "EMPTY"
	started := false
	currentTime := ""

	lg := NewXmlLogger()
	lg.Launch("argocd-e2e-186_last", "")
	i := 0
	wc := script.Exec("cat /workdir/test-results/argocd-e2e-186_last.log").FilterScan(func(line string, w io.Writer) {
		i++
		if i%100 == 0 {
			fmt.Printf("Processing line %v\n", i)
		}
		if m := getMatches(reRUN, line); len(m) > 0 {
			currentTest = m["name"]
			started = true
		}
		if m := getMatches(reLOG, line); len(m) > 0 {
			currentTime = m["time"]
			if started {
				lg.StartTest(currentTest, currentTime)
				started = false
			}
			lg.AddLine(m["timestamp"], m["level"], m["msg"])
		}
		if m := getMatches(reEND, line); len(m) > 0 {
			if started {
				lg.StartTest(currentTest, currentTime)
				started = false
			}
			e := m["result"]
			lg.FinnishTest(e, m["time"])
		}
	}).Stdout()
	output, err := xml.MarshalIndent(lg, "  ", "    ")
	if err != nil {
		fmt.Printf("error: %v\n", err)
	} else {
		script.Echo(string(output[:])).WriteFile("/workdir/test-results/argocd-e2e-186_last.xml")
	}
}

func main() {
	fmt.Printf("Running project: `%s`\n", src.ProjectName())

	// These functions demonstrate two separate checks to detect if the code is being
	// run inside a docker container in debug mode, or production mode!
	//
	// Note: Valid only for docker containers generated using the Makefile command
	firstCheck()
	secondCheck()
}

func firstCheck() bool {
	/*
	 * The `debug_mode` environment variable exists only in debug builds, likewise,
	 * `production_mode` variable exists selectively in production builds - use the
	 * existence of these variables to detect container build type (and not values)
	 *
	 * Exactly one of these - `production_mode` or `debug_mode` - is **guaranteed** to
	 * exist for docker builds generated using the Makefile commands!
	 */

	if _, ok := os.LookupEnv("production_mode"); ok {
		fmt.Println("(Check 01): Running in `production` mode!")
		return true
	} else if _, ok := os.LookupEnv("debug_mode"); ok {
		// Could also use a simple `else` statement (above) for docker builds!
		fmt.Println("(Check 01): Running in `debug` mode!")
		return true
	} else {
		fmt.Println("\nP.S. Try running a build generated with the Makefile :)")
		return false
	}
}

func secondCheck() bool {
	/*
	 * There's also a central `__BUILD_MODE__` variable for a dirty checks -- guaranteed
	 * to exist for docker containers generated by the Makefile commands!
	 * The variable will have a value of `production` or `debug` (no capitalization)
	 *
	 * Note: Relates to docker builds generated using the Makefile
	 */

	value := os.Getenv("__BUILD_MODE__")

	// Yes, this if/else could have been written better
	switch value {
	case "production":
		fmt.Println("(Check 02): Running in `production` mode!")
		return true

	case "debug":
		fmt.Println("(Check 02): Running in `debug` mode!")
		return true

	default:
		// Flow ends up here for non-docker builds, or docker builds generated directly
		fmt.Println("Non-makefile build detected :(")
		return false
	}
}
