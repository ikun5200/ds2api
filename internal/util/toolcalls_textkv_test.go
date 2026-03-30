package util

import (
	"testing"
)

func TestParseTextKVToolCalls_Basic(t *testing.T) {
	text := `
status: already_called
origin: assistant
not_user_input: true
tool_call_id: call_3fcd15235eb94f7eae3a8de5a9cfa36b
function.name: execute_command
function.arguments: {"command":"cd scripts && python check_syntax.py example.py","cwd":null,"timeout":30}

Some other text thinking...
`
	calls := ParseToolCalls(text, []string{"execute_command"})
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "execute_command" {
		t.Fatalf("unexpected name: %s", calls[0].Name)
	}
	if calls[0].Input["command"] != "cd scripts && python check_syntax.py example.py" {
		t.Fatalf("unexpected command arg: %v", calls[0].Input["command"])
	}
}

func TestParseTextKVToolCalls_Multiple(t *testing.T) {
	text := `
function.name: read_file
function.arguments: {
	"path": "abc.txt"
}

function.name: bash
function.arguments: {"command": "ls"}
`
	calls := ParseToolCalls(text, []string{"read_file", "bash"})
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if calls[0].Name != "read_file" {
		t.Fatalf("unexpected 1st name: %s", calls[0].Name)
	}
	if calls[1].Name != "bash" {
		t.Fatalf("unexpected 2nd name: %s", calls[1].Name)
	}
}

func TestParseTextKVToolCalls_Standalone(t *testing.T) {
	text := "function.name: read_file\nfunction.arguments: {\"path\":\"README.md\"}"
	calls := ParseStandaloneToolCalls(text, []string{"read_file"})
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "read_file" {
		t.Fatalf("unexpected name: %s", calls[0].Name)
	}
}
