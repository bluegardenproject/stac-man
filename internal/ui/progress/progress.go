// Package progress renders incremental status for long-running CLI
// operations (push to origin, gh round-trips, restack walks). It picks
// an animated spinner when stdout is a TTY with color enabled and falls
// back to plain "✓ msg" / "✗ msg" lines in pipes or NO_COLOR
// environments, so the same service code can drive both interactive
// and scripted runs without conditional output paths at every call
// site.
//
// The Reporter contract is intentionally tiny — Start opens a step,
// exactly one of Done / Fail / Skip closes it. A second Start before
// the previous step is closed auto-finishes the prior one as Done so
// callers can chain steps without bookkeeping. Reporters are
// safe for sequential use within a single command but are NOT designed
// for concurrent steps; submit, log, and doctor all run their gh
// round-trips serially today.
package progress

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/bluegardenproject/stac-man/internal/ui"
	"github.com/bluegardenproject/stac-man/internal/ui/theme"
)

// Reporter receives one Start per long-running step and exactly one
// terminal call (Done / Fail / Skip) for that step.
//
// A nil Reporter is NOT safe — callers should pass progress.Discard()
// when they want to suppress output (machine-readable formats, tests).
// The cmd layer always wires up either a spinner, a plain reporter,
// or Discard().
type Reporter interface {
	// Start opens a new step with the given label. Spinner reporters
	// begin animating; plain reporters do nothing until the step
	// closes.
	Start(label string)
	// Done closes the current step as success. label may be empty to
	// reuse the Start label, or different to summarise the outcome
	// ("pushed feat/foo" after Start("pushing feat/foo")).
	Done(label string)
	// Fail closes the current step as failure. The error itself is
	// expected to bubble up through the normal return path; this
	// just paints the line in the failure colour.
	Fail(label string)
	// Skip closes the current step as a no-op (e.g. branch already
	// in sync with origin) so the user sees we considered it but
	// didn't need to do work.
	Skip(label string)
}

// New picks the right reporter for out. Returns the spinner reporter
// when out is a TTY with color enabled, the plain reporter otherwise,
// and Discard() when out is nil. Callers wanting to force-disable
// (machine-readable output) should pass Discard() directly instead.
func New(out io.Writer) Reporter {
	if out == nil {
		return Discard()
	}
	if !ui.ColorEnabled() {
		return &plainReporter{w: out}
	}
	if f, ok := out.(*os.File); ok {
		if fi, err := f.Stat(); err != nil || (fi.Mode()&os.ModeCharDevice) == 0 {
			return &plainReporter{w: out}
		}
	} else {
		// Non-*os.File writers (test buffers, captured pipes) can't
		// animate cleanly. Plain reporter keeps the output stable.
		return &plainReporter{w: out}
	}
	return newSpinnerReporter(out)
}

// Discard returns a Reporter that drops every event. Use in tests, in
// `--json` / `--porcelain` code paths where progress output would
// corrupt the machine-readable stream, and any other "I want the
// service quiet" call site.
func Discard() Reporter { return discardReporter{} }

type discardReporter struct{}

func (discardReporter) Start(string) {}
func (discardReporter) Done(string)  {}
func (discardReporter) Fail(string)  {}
func (discardReporter) Skip(string)  {}

// plainReporter prints one line per terminal event (Done / Fail /
// Skip). Start is a no-op so the output reads as a quiet stream of
// completed steps — matching what `git push` and `gh` print when
// stdout is piped — instead of two lines per step in non-interactive
// shells.
type plainReporter struct {
	w  io.Writer
	mu sync.Mutex
}

func (r *plainReporter) Start(string) {}

func (r *plainReporter) Done(label string) { r.line("✓", label) }
func (r *plainReporter) Fail(label string) { r.line("✗", label) }
func (r *plainReporter) Skip(label string) { r.line("·", label) }

func (r *plainReporter) line(mark, label string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fmt.Fprintf(r.w, "%s %s\n", mark, label)
}

// spinnerReporter draws an animated braille spinner on the current
// line until the step finishes, then replaces the line with the final
// mark + label so each step occupies exactly one row of scrollback.
//
// A goroutine started in Start drives the animation via a ticker; the
// goroutine reads frame state through the mutex so a Done in flight
// can't race with the next frame draw. A Start that arrives while a
// step is still active auto-finishes the prior step as Done — the
// service layer sometimes hits "skipped a branch, on to the next" and
// we don't want to require an explicit close in that case.
type spinnerReporter struct {
	w        io.Writer
	frames   []string
	interval time.Duration

	mu     sync.Mutex
	label  string
	done   chan struct{}
	active bool
}

func newSpinnerReporter(w io.Writer) *spinnerReporter {
	return &spinnerReporter{
		w:        w,
		frames:   []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		interval: 80 * time.Millisecond,
	}
}

func (r *spinnerReporter) Start(label string) {
	r.finish("✓", "", true)

	r.mu.Lock()
	r.label = label
	r.done = make(chan struct{})
	r.active = true
	r.draw(r.frames[0])
	done := r.done
	r.mu.Unlock()

	go r.loop(done)
}

func (r *spinnerReporter) loop(done chan struct{}) {
	t := time.NewTicker(r.interval)
	defer t.Stop()
	i := 1
	for {
		select {
		case <-done:
			return
		case <-t.C:
			r.mu.Lock()
			if r.active {
				r.draw(r.frames[i%len(r.frames)])
			}
			r.mu.Unlock()
			i++
		}
	}
}

func (r *spinnerReporter) Done(label string) { r.finish("✓", label, false) }
func (r *spinnerReporter) Fail(label string) { r.finish("✗", label, false) }
func (r *spinnerReporter) Skip(label string) { r.finish("·", label, false) }

// finish closes the active step (if any) and prints a final line with
// the given mark. If finalLabel is empty, the Start label is reused.
// soft=true marks an auto-close from a chained Start: we suppress the
// terminal line because the next Start is about to draw immediately
// over the same row, so emitting here would just create an empty
// scrollback line that the next draw clobbers anyway.
//
// Calling a terminal method (Done / Fail / Skip) without an active
// Start is supported: callers like the in-sync push branch want to
// emit "X already in sync" without paying for a spinner first. In
// that case finalLabel is required (we have no Start label to fall
// back on); an empty label is silently dropped.
func (r *spinnerReporter) finish(mark, finalLabel string, soft bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active {
		if finalLabel == "" {
			finalLabel = r.label
		}
		r.active = false
		close(r.done)
		if soft {
			return
		}
	}
	if finalLabel == "" {
		return
	}
	fmt.Fprintf(r.w, "\r\x1b[2K%s %s\n", styledMark(mark), finalLabel)
}

// draw paints the current frame on stdout in-place (\r resets the
// cursor; \x1b[2K clears the line) so the spinner doesn't accumulate
// scrollback while it's animating. Caller holds r.mu.
func (r *spinnerReporter) draw(frame string) {
	fmt.Fprintf(r.w, "\r\x1b[2K%s %s",
		ui.Render(theme.Accent, frame), r.label)
}

// styledMark colours the terminal mark. Pulled out so the plain
// reporter (which never colours) and the spinner reporter share the
// same glyph vocabulary while only the spinner pays the lipgloss cost.
func styledMark(m string) string {
	switch m {
	case "✓":
		return ui.Render(theme.OK, m)
	case "✗":
		return ui.Render(theme.Fail, m)
	default:
		return ui.Render(theme.Dimmed, m)
	}
}
