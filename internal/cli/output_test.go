package cli

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/event"
)

// captureFDs runs fn with os.Stdout and os.Stderr replaced by pipes and returns
// what each received. Tests that inject writers cannot catch this class of bug:
// cobra's cmd.Print* family resolves through OutOrStderr(), which falls back to
// os.Stderr precisely when no writer was injected — that is, in production.
func captureFDs(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	origOut, origErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout, os.Stderr = outW, errW

	done := make(chan struct{})
	var outBuf, errBuf bytes.Buffer
	go func() {
		_, _ = io.Copy(&outBuf, outR)
		close(done)
	}()
	errDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&errBuf, errR)
		close(errDone)
	}()

	fn()

	outW.Close()
	errW.Close()
	<-done
	<-errDone
	os.Stdout, os.Stderr = origOut, origErr
	return outBuf.String(), errBuf.String()
}

// Adapters run `kb reconcile --json` and pipe stdout to jq. If the document goes
// to stderr the caller captures nothing and the integration silently does nothing.
func TestReconcileJSON_reaches_real_stdout(t *testing.T) {
	t.Setenv("KB_RECONCILING", "")
	d := db.OpenTest(t)
	testDB = d
	t.Cleanup(func() { testDB = nil })
	if _, err := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "x", Source: "test",
		SessionID: "ses", Status: event.StatusInProgress,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	stdout, stderr := captureFDs(t, func() {
		root := NewRoot()
		root.SetArgs([]string{"reconcile", "--project", "proj", "--session-id", "ses", "--json"})
		if err := root.Execute(); err != nil {
			t.Errorf("reconcile: %v", err)
		}
	})

	if !bytes.Contains([]byte(stdout), []byte(`"schema_version"`)) {
		t.Errorf("schema-v1 document not on stdout.\nstdout=%q\nstderr=%q", stdout, stderr)
	}
}

func TestListOutput_reaches_real_stdout(t *testing.T) {
	d := db.OpenTest(t)
	testDB = d
	t.Cleanup(func() { testDB = nil })
	if _, err := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "pipeable", Source: "test",
		Status: event.StatusInProgress,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	stdout, stderr := captureFDs(t, func() {
		root := NewRoot()
		root.SetArgs([]string{"list", "--project", "proj"})
		if err := root.Execute(); err != nil {
			t.Errorf("list: %v", err)
		}
	})

	if !bytes.Contains([]byte(stdout), []byte("pipeable")) {
		t.Errorf("task not on stdout.\nstdout=%q\nstderr=%q", stdout, stderr)
	}
}
