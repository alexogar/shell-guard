package api

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestWriteRequestAndReadBack(t *testing.T) {
	var buf bytes.Buffer
	req := Request{Action: ActionGetState, Limit: 3}
	if err := writeRequest(&buf, req); err != nil {
		t.Fatalf("writeRequest failed: %v", err)
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Fatalf("request should end with newline: %q", buf.String())
	}
}

func TestReadResponse(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(`{"ok":true,"message":"ready"}` + "\n"))
	resp, err := readResponse(reader)
	if err != nil {
		t.Fatalf("readResponse failed: %v", err)
	}
	if !resp.OK || resp.Message != "ready" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestReadResponseRejectsInvalidJSON(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(`{nope}` + "\n"))
	if _, err := readResponse(reader); err == nil {
		t.Fatal("expected invalid json to fail")
	}
}
