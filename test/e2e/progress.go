package e2e

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-isatty"
)

const (
	defaultProgressHeartbeat = 30 * time.Second
	defaultProgressTick      = time.Second
)

// scenarioProgress renders compact "[completed/total] elapsed <dur> · current: <name>"
// progress for an in-flight E2E scenario run: one redrawn line on interactive
// terminals, periodic lines (with a heartbeat during long scenarios) otherwise.
type scenarioProgress struct {
	out         io.Writer
	interactive bool
	total       int
	now         func() time.Time
	heartbeat   time.Duration
	manualTick  bool

	mu        sync.Mutex
	start     time.Time
	completed int
	current   string
	lastEmit  time.Time
	printed   bool

	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}
}

type progressOption func(*scenarioProgress)

// withProgressClock injects a fake clock so tests can control elapsed time deterministically.
func withProgressClock(now func() time.Time) progressOption {
	return func(p *scenarioProgress) { p.now = now }
}

func withProgressHeartbeat(heartbeat time.Duration) progressOption {
	return func(p *scenarioProgress) { p.heartbeat = heartbeat }
}

// withManualTick disables the background ticker so tests can drive tick() calls directly.
func withManualTick() progressOption {
	return func(p *scenarioProgress) { p.manualTick = true }
}

func newScenarioProgress(out io.Writer, total int, interactive bool, opts ...progressOption) *scenarioProgress {
	p := &scenarioProgress{
		out:         out,
		interactive: interactive,
		total:       total,
		now:         time.Now,
		heartbeat:   defaultProgressHeartbeat,
		stopCh:      make(chan struct{}),
		doneCh:      make(chan struct{}),
	}
	for _, opt := range opts {
		opt(p)
	}
	p.start = p.now()
	p.lastEmit = p.start

	if p.manualTick {
		close(p.doneCh)
		return p
	}
	tickEvery := p.heartbeat
	if p.interactive {
		tickEvery = defaultProgressTick
	}
	go p.loop(tickEvery)
	return p
}

func (p *scenarioProgress) loop(interval time.Duration) {
	defer close(p.doneCh)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.tick()
		}
	}
}

// tick re-renders progress. Interactive terminals redraw on every tick; non-interactive
// output only emits a heartbeat line once the configured interval has elapsed.
func (p *scenarioProgress) tick() {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	if !p.interactive && now.Sub(p.lastEmit) < p.heartbeat {
		return
	}
	p.emitLocked(now)
}

// startScenario marks the named scenario as running and immediately emits progress.
func (p *scenarioProgress) startScenario(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current = name
	p.emitLocked(p.now())
}

// completeScenario records that the current scenario finished (pass, fail, or skip).
func (p *scenarioProgress) completeScenario() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completed++
}

func (p *scenarioProgress) emitLocked(now time.Time) {
	line := formatProgressLine(p.completed, p.total, now.Sub(p.start), p.current)
	if p.interactive {
		fmt.Fprintf(p.out, "\r\x1b[2K%s", line)
	} else {
		fmt.Fprintln(p.out, line)
	}
	p.lastEmit = now
	p.printed = true
}

// stop halts background updates and, on interactive terminals, terminates the
// in-place line so later output starts on its own line.
func (p *scenarioProgress) stop() {
	p.stopOnce.Do(func() { close(p.stopCh) })
	<-p.doneCh

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.interactive && p.printed {
		fmt.Fprint(p.out, "\n")
	}
}

func formatProgressLine(completed, total int, elapsed time.Duration, current string) string {
	return fmt.Sprintf("[%d/%d] elapsed %s · current: %s", completed, total, formatElapsed(elapsed), current)
}

// formatElapsed renders a duration rounded to whole seconds, e.g. "6m32s".
func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	return d.Round(time.Second).String()
}

// progressScenarioName renders the compact scenario name shown as "current" in
// progress output, matching the normalized names used for shard/replay bookkeeping.
func progressScenarioName(scenarioPath string) string {
	return strings.TrimSuffix(normalizeScenarioPath(scenarioPath), "/scenario.yaml")
}

// isInteractiveWriter reports whether f is a real terminal. CI and the local
// Makefile both stream test output through tee, so the underlying fd is a pipe
// there and this correctly reports non-interactive even inside a CI terminal.
func isInteractiveWriter(f interface{ Fd() uintptr }) bool {
	fd := f.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}
