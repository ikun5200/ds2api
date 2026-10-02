package testsuite

import (
	"reflect"
	"testing"
)

func TestPreflightStepsExactSequence(t *testing.T) {
	want := [][]string{
		{"go", "test", "./...", "-count=1"},
		{"./tests/scripts/check-node-split-syntax.sh"},
		{"node", "--test", "tests/node/chat-history-utils.test.js", "tests/node/api-tester-attachments.test.js"},
		{"npm", "run", "build", "--prefix", "webui"},
	}

	got := preflightSteps()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("preflight steps mismatch\nwant=%v\ngot=%v", want, got)
	}
}
