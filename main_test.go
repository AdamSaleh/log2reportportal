package main

import (
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	envProd  = "production_mode"
	envDebug = "debug_mode"
	envBuild = "__BUILD_MODE__"
)

var _ = Describe("DefaultLines and getMatches", func() {
	dl := &DefaultLines{}

	DescribeTable("Matching withGetMatches", func(r, line string, expected map[string]string) {
		re := regexp.MustCompile(r)
		actual := getMatches(re, line)
		Expect(actual).To(Equal(expected))
	},
		Entry(dl.reEND, "--- PASS: TestCreateAndUseAccount (5.56s)", map[string]string{"name": "TestCreateAndUseAccount", "test": "PASS", "time": "5.56"}),
		Entry(dl.reSTAMP, `  startTime: "2023-11-21T00:17:10Z"`, map[string]string{"startDate": "2023-11-21"}),
	)
})
