package progress

import (
	"bytes"
	"strings"
	"testing"
)

// TestPlainReporter_StartIsSilent_DoneEmitsLine pins down the contract
// the cmd layer relies on for piped output: Start writes nothing, only
// the terminal call (Done/Fail/Skip) emits a single line. That keeps
// `sm submit | tee` clean — one line per finished step, no half-state
// sentinels in scrollback.
func TestPlainReporter_StartIsSilent_DoneEmitsLine(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := &plainReporter{w: &buf}

	r.Start("pushing feat/foo")
	if buf.Len() != 0 {
		t.Fatalf("plain reporter emitted on Start: %q", buf.String())
	}

	r.Done("pushed feat/foo")
	got := buf.String()
	if got != "✓ pushed feat/foo\n" {
		t.Fatalf("Done line = %q, want %q", got, "✓ pushed feat/foo\n")
	}
}

// TestPlainReporter_FailAndSkip_HaveDistinctMarks documents the glyph
// vocabulary so the cmd layer (and screenshots in the docs) can rely
// on visual distinction between failure, skip, and success.
func TestPlainReporter_FailAndSkip_HaveDistinctMarks(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := &plainReporter{w: &buf}

	r.Fail("creating PR for feat/bar")
	r.Skip("feat/baz already in sync with origin")

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "✗ ") {
		t.Errorf("Fail line = %q, want ✗ prefix", lines[0])
	}
	if !strings.HasPrefix(lines[1], "· ") {
		t.Errorf("Skip line = %q, want · prefix", lines[1])
	}
}

// TestDiscard_DropsEverything ensures the reporter handed to --json /
// --porcelain output paths really is silent — any leak would corrupt
// the machine-readable stream those formats promise.
func TestDiscard_DropsEverything(t *testing.T) {
	t.Parallel()
	r := Discard()

	r.Start("anything")
	r.Done("anything")
	r.Fail("anything")
	r.Skip("anything")
	// no observable output channel; method calls completing without
	// panic is the whole contract.
}

// TestNew_NilWriterReturnsDiscard locks the contract used by callers
// that pass `nil` when they explicitly want progress disabled (e.g.
// passing a Discard via os.Stdout=nil shorthand). Without this guard
// New would deref nil inside ColorEnabled's TTY check.
func TestNew_NilWriterReturnsDiscard(t *testing.T) {
	t.Parallel()
	got := New(nil)
	if _, ok := got.(discardReporter); !ok {
		t.Fatalf("New(nil) = %T, want discardReporter", got)
	}
}

// TestNew_NonFileWriterFallsBackToPlain pins the rule that test
// buffers, captured pipes, and any io.Writer that isn't an *os.File
// get the plain reporter — animating into a bytes.Buffer would mean
// snapshot tests had to scrub ANSI escapes on every assertion.
func TestNew_NonFileWriterFallsBackToPlain(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	got := New(&buf)
	if _, ok := got.(*plainReporter); !ok {
		t.Fatalf("New(*bytes.Buffer) = %T, want *plainReporter", got)
	}
}
