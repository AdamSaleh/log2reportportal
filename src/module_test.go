package src

import (
	"testing"
)

func TestModuleName(t *testing.T) {
	if ProjectName() != "log2reportportal" {
		t.Errorf("Project name `%s` incorrect", ProjectName())
	}
}
