package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"testing"

	"github.com/bitfield/script"
	"github.com/go-resty/resty/v2"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var client *resty.Client

var _ = BeforeSuite(func() {
	// block all HTTP requests
	client = resty.New()
	httpmock.ActivateNonDefault(client.GetClient())
})

var _ = BeforeEach(func() {
	// remove any mocks
	httpmock.Reset()
})

var _ = AfterSuite(func() {
	httpmock.DeactivateAndReset()
})

type CasesType map[string]map[string][]map[string]string

type MockReportBuilder struct {
	Cases       CasesType `json:"cases,omitempty"`
	LaunchName  string    `json:"launchName,omitempty"`
	StartStamp  string    `json:"startStamp,omitempty"`
	FinishStamp string    `json:"finishStamp,omitempty"`
}

func (m *MockReportBuilder) getLaunch(name string) int {
	if m.LaunchName != "" {
		return 1
	} else {
		return -1
	}
}

func (m *MockReportBuilder) getCase(name string) int {
	if _, ok := m.Cases[name]; !ok {
		return -1
	}
	return 1
}

func (m *MockReportBuilder) EnsureTest(name, startTime string) {
	if m.getCase(name) >= 0 {
		return
	}
	m.Cases[name] = map[string][]map[string]string{
		startTime: {{"c": "StartTest"}},
	}
}

func (m *MockReportBuilder) AddLine(name, startTime, level, message string) {
	m.Cases[name][startTime] = append(m.Cases[name][startTime], map[string]string{
		"msg": message,
	})
}

func (m *MockReportBuilder) FinnishTest(name, result, time string) {
	m.Cases[name]["finished"] = []map[string]string{
		{"result": result, "time": time},
	}
}

func (m *MockReportBuilder) Finish(time string) {
	m.FinishStamp = time
}

func (m *MockReportBuilder) EnsureLaunch(name, startTime string) {
	if m.getLaunch(name) >= 0 {
		return
	}
	m.LaunchName = name
	m.StartStamp = startTime
}

func mapToMatcher(m CasesType) Keys {
	o := Keys{}
	for k, v := range m {
		o1 := Keys{}
		for k1, v1 := range v {
			o1[k1] = Equal(v1)
		}
		o[k] = MatchAllKeys(o1)
	}
	return o
}

func mockOkJSON(v any) httpmock.Responder {
	return httpmock.NewJsonResponderOrPanic(
		200,
		v)
}

type TestRestLogger struct {
	log string
}

func (l *TestRestLogger) Errorf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	s, _ = script.Echo(s).ReplaceRegexp(regexp.MustCompile("^RECEIVED AT.*$"), "").String()
	l.log = fmt.Sprintf("%s\n\n%s", l.log, s)
}

func (l *TestRestLogger) Warnf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	s, _ = script.Echo(s).ReplaceRegexp(regexp.MustCompile("^RECEIVED AT.*$"), "").String()
	l.log = fmt.Sprintf("%s\n\n%s", l.log, s)
}

func (l *TestRestLogger) Debugf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	s, _ = script.Echo(s).
		ReplaceRegexp(regexp.MustCompile("^RECEIVED AT.*$"), "").
		ReplaceRegexp(regexp.MustCompile("^TIME DURATION.*$"), "").
		String()
	l.log = fmt.Sprintf("%s\n\n%s", l.log, s)
}

var _ = Describe("Testing parsing", func() {
	dl := &DefaultLines{}

	DescribeTable("Processing with MockReportBuilder", func(inputFile, expectedtFile string) {
		actual := &MockReportBuilder{Cases: map[string]map[string][]map[string]string{}}
		processLinear(actual, "TestName", script.File(inputFile))
		file, err := os.OpenFile(expectedtFile, os.O_RDONLY, 0o666)
		if errors.Is(err, os.ErrNotExist) {
			b, errM := json.MarshalIndent(actual, "", "    ")
			Expect(errM).To(BeNil())
			out := string(b)
			_, errW := script.Echo(out).WriteFile(expectedtFile)
			Expect(errW).To(BeNil())
		} else {
			Expect(err).To(BeNil())
			defer file.Close()
			bytes, _ := ioutil.ReadAll(file)
			expected := &MockReportBuilder{Cases: CasesType{}}
			errU := json.Unmarshal(bytes, expected)
			Expect(errU).To(BeNil())
			Expect(actual.Cases).To(MatchAllKeys(mapToMatcher(expected.Cases)))
		}
	},
		Entry("Test parse kuttl-parllel",
			"./test_data/parallel-kuttl.txt", "./test_data/parallel-kuttl.json"),
		Entry("Test parse argocd-e2e",
			"./test_data/argocd-e2e-186_last.log", "./test_data/argocd-e2e-186_last.json"),
	)

	DescribeTable("Matching withGetMatches",
		func(r, line string, expected Keys) {
			re := regexp.MustCompile(r)
			actual := getMatches(re, line)
			Expect(actual).To(MatchAllKeys(expected))
		},
		Entry("Test reEND",
			dl.reEND(), "--- PASS: TestCreateAndUseAccount (5.56s)",
			Keys{
				"test":     Equal("TestCreateAndUseAccount"),
				"result":   Equal("PASS"),
				"duration": Equal("5.56"),
			}),
		Entry("Test reSTAMP",
			dl.reSTAMP(), `  startTime: "2023-11-21T00:17:10Z"`,
			Keys{"startDate": Equal("2023-11-21")}),
	)
})

var _ = Describe("Testing mock upload", func() {
	It("Exported", func() {
		client.SetBaseURL("http://portal/")
		client.SetDebug(true)
		l := &TestRestLogger{log: ""}
		client.SetLogger(l)
		tr := map[string]string{"id": "testid"}
		tu := map[string]string{"uuid": "testid"}

		httpmock.RegisterResponder(
			"POST",
			"http://portal/api/v1/TEST_PROJECT/launch",
			mockOkJSON(tr))
		httpmock.RegisterResponder(
			"GET",
			"http://portal/api/v1/TEST_PROJECT/launch/testid",
			mockOkJSON(tu))
		httpmock.RegisterResponder(
			"PUT",
			"http://portal/api/v1/TEST_PROJECT/launch/testid/finish/",
			mockOkJSON(tr))

		httpmock.RegisterResponder(
			"POST",
			"http://portal/api/v2/TEST_PROJECT/item",
			mockOkJSON(tr))

		httpmock.RegisterResponder(
			"PUT",
			"http://portal/api/v1/TEST_PROJECT/item/testid",
			mockOkJSON(tr))

		httpmock.RegisterResponder(
			"POST",
			"http://portal/api/v2/TEST_PROJECT/log/entry",
			mockOkJSON(tr))

		lg := NewRPLogger(client, "TOKEN", "TEST_PROJECT")

		processLinear(lg, "REPORT_NAME", script.File("./test_data/minimal-kuttl.txt"))
		file, err := os.OpenFile("./test_data/http_log", os.O_RDONLY, 0o666)
		if errors.Is(err, os.ErrNotExist) {
			_, errW := script.Echo(l.log).WriteFile("./test_data/http_log")
			Expect(errW).To(BeNil())
		} else {
			Expect(err).To(BeNil())
			defer file.Close()
			bytes, _ := ioutil.ReadAll(file)
			Expect(l.log).To(Equal(string(bytes)))
		}
	})
})

func TestAll(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Uploader Suite")
}
