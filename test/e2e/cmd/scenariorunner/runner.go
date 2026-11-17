package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

const (
	defaultHeartbeat    = 30 * time.Second
	defaultTickInterval = time.Second
)

// runConfig configures one scenario run.
type runConfig struct {
	artifactsDir string
	testArgs     []string // flags/package passed straight through to `go test`, after "-json"
	interactive  bool
	now          func() time.Time
	heartbeat    time.Duration
	tickInterval time.Duration
}

// driveConfig configures the event-stream-to-renderer pipeline, decoupled
// from process spawning so it can be exercised with fixture readers in tests.
type driveConfig struct {
	stdout       io.Reader
	stderr       io.Reader
	runLog       io.Writer
	renderer     *Renderer
	tick         func(func(time.Time)) func() // installs a tick callback, returns a stop func
	artifactsDir string
}

// drive consumes the test2json stdout stream and raw stderr lines from a
// scenario run, feeding the renderer and run.log, until both reach EOF. It
// returns the final summary and whether Test_Scenarios ever reached a
// terminal action (false means the run ended abnormally: a panic, a timeout,
// or the process was killed).
func drive(cfg driveConfig) (summary, bool) {
	agg := newAggregator()
	var runLogMu sync.Mutex
	appendRunLog := func(s string) {
		if cfg.runLog == nil || s == "" {
			return
		}
		runLogMu.Lock()
		defer runLogMu.Unlock()
		_, _ = io.WriteString(cfg.runLog, s)
	}

	var stopTick func()
	if cfg.tick != nil {
		stopTick = cfg.tick(cfg.renderer.tick)
		defer stopTick()
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(cfg.stderr)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			cfg.renderer.PermanentLine(line)
			appendRunLog(line + "\n")
		}
	}()

	go func() {
		defer wg.Done()
		_ = decodeEvents(cfg.stdout, func(ev testEvent) {
			if ev.Action == "output" || ev.Action == "build-output" {
				appendRunLog(ev.Output)
			}
			for _, u := range agg.apply(ev) {
				cfg.renderer.onUpdate(u)
			}
		})
	}()

	wg.Wait()

	return agg.summary(cfg.artifactsDir), agg.runDone
}

// tickerFunc returns a tick installer that drives fn from a real time.Ticker
// at the given interval, for production use.
func tickerFunc(interval time.Duration, now func() time.Time) func(func(time.Time)) func() {
	return func(fn func(time.Time)) func() {
		ticker := time.NewTicker(interval)
		done := make(chan struct{})
		go func() {
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					fn(now())
				}
			}
		}()
		return func() {
			ticker.Stop()
			close(done)
		}
	}
}

// runScenarios execs `go test -json <testArgs...>`, renders progress to out,
// writes run.log under artifactsDir, and returns the exit code `go test`
// itself exited with.
func runScenarios(cfg runConfig, out io.Writer) (int, error) {
	runLogPath := cfg.artifactsDir + string(os.PathSeparator) + "run.log"
	runLog, err := os.OpenFile(runLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open run.log: %w", err)
	}
	defer func() { _ = runLog.Close() }()

	// #nosec G204 -- cfg.testArgs are the go test flags/package Makefile already passes directly to `go test`
	cmd := exec.Command("go", append([]string{"test", "-json"}, cfg.testArgs...)...)
	cmd.Env = append(os.Environ(), "KONGCTL_E2E_COMPACT=1")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return 0, fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start go test: %w", err)
	}

	renderer := newRenderer(out, cfg.interactive, cfg.now, cfg.heartbeat)
	s, ranToCompletion := drive(driveConfig{
		stdout:       stdout,
		stderr:       stderr,
		runLog:       runLog,
		renderer:     renderer,
		tick:         tickerFunc(cfg.tickInterval, cfg.now),
		artifactsDir: cfg.artifactsDir,
	})

	waitErr := cmd.Wait()

	if !ranToCompletion {
		if waitErr != nil {
			renderer.Abnormal(waitErr)
		} else {
			renderer.Abnormal(errors.New("process exited without completing"))
		}
	}
	renderer.Finish(s)

	if waitErr == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return 0, fmt.Errorf("wait go test: %w", waitErr)
}
