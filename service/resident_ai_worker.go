package service

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type residentWorkerStats struct {
	Reused    bool
	StartupMs int64
	RequestMs int64
}

func (a *HybridAnalyzer) lastWorkerStats() residentWorkerStats {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.workerStats
}

type residentAIWorker struct {
	identity [32]byte
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	reader   *io.PipeReader
	scanner  *bufio.Scanner
	done     chan struct{}
	stopOnce sync.Once
}

func (w *residentAIWorker) abort() {
	w.stopOnce.Do(func() {
		_ = w.stdin.Close()
		terminateAICommand(w.cmd)
		_ = w.reader.Close()
	})
}

func (a *HybridAnalyzer) Close() error {
	a.mu.Lock()
	a.closed = true
	w := a.worker
	a.worker = nil
	a.mu.Unlock()
	if w != nil {
		w.abort()
		<-w.done
	}
	return nil
}

func (a *HybridAnalyzer) discard(w *residentAIWorker) {
	a.mu.Lock()
	if a.worker == w {
		a.worker = nil
	}
	a.mu.Unlock()
	w.abort()
}

func (a *HybridAnalyzer) runResident(ctx context.Context, python, root string, payload []byte, onEvent func(AIEvent)) error {
	requestStarted := time.Now()
	stats := residentWorkerStats{}
	defer func() {
		stats.RequestMs = time.Since(requestStarted).Milliseconds()
		a.mu.Lock()
		a.workerStats = stats
		a.mu.Unlock()
	}()
	env := aiWorkerEnvironment(os.Environ())
	identity := sha256.Sum256([]byte(python + "\n" + root + "\n" + strings.Join(env, "\n")))
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return errors.New("local AI worker is closed")
	}
	w := a.worker
	if w != nil {
		select {
		case <-w.done:
			w.abort()
			w = nil
			a.worker = nil
		default:
		}
	}
	if w != nil && w.identity != identity {
		w.abort()
		w = nil
		a.worker = nil
	}
	stats.Reused = w != nil
	if w == nil {
		command := a.command
		if command == nil {
			command = exec.Command
		}
		cmd := command(python, "-u", filepath.Join(root, "tools", "app_caption_worker.py"), "--serve")
		configureAICommand(cmd)
		cmd.Dir, cmd.Env = root, env
		reader, writer := io.Pipe()
		cmd.Stdout = writer
		// Provider/model diagnostics are not copied into UI errors or persisted logs.
		stdin, err := cmd.StdinPipe()
		if err != nil {
			a.mu.Unlock()
			_ = reader.Close()
			_ = writer.Close()
			return err
		}
		if err := cmd.Start(); err != nil {
			a.mu.Unlock()
			_ = stdin.Close()
			_ = reader.Close()
			_ = writer.Close()
			return fmt.Errorf("start local AI worker: %w", err)
		}
		w = &residentAIWorker{identity: identity, cmd: cmd, stdin: stdin, reader: reader, scanner: bufio.NewScanner(reader), done: make(chan struct{})}
		w.scanner.Buffer(make([]byte, 4096), 2<<20)
		a.worker = w
		go func(worker *residentAIWorker) {
			err := worker.cmd.Wait()
			_ = writer.CloseWithError(err)
			close(worker.done)
		}(w)
	}
	a.mu.Unlock()
	jobDone := make(chan struct{})
	watcherDone := make(chan struct{})
	defer func() { close(jobDone); <-watcherDone }()
	go func() {
		defer close(watcherDone)
		select {
		case <-ctx.Done():
			select {
			case <-jobDone:
				return
			default:
			}
			w.abort()
		case <-jobDone:
		}
	}()
	if _, err := w.stdin.Write(append(payload, '\n')); err != nil {
		a.discard(w)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("local AI worker stopped; completed captions are preserved, try again")
	}
	var workerErr error
	for w.scanner.Scan() {
		var event AIEvent
		if err := json.Unmarshal(w.scanner.Bytes(), &event); err != nil {
			a.discard(w)
			return errors.New("invalid local AI worker response")
		}
		switch event.Type {
		case "ready":
			if !stats.Reused {
				stats.StartupMs = time.Since(requestStarted).Milliseconds()
			}
		case "error":
			workerErr = errors.New(event.Message)
		case "done":
			if ctx.Err() != nil {
				a.discard(w)
				return ctx.Err()
			}
			return workerErr
		case "progress", "result", "failure":
			onEvent(event)
		default:
			a.discard(w)
			return errors.New("unknown local AI worker event")
		}
	}
	a.discard(w)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return errors.New("local AI worker stopped; completed captions are preserved, try again")
}
