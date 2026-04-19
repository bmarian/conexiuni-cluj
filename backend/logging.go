package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const StandardLogTimestampLayout = "2006/01/02 15:04:05"

type rotatingWriter struct {
	mu          sync.Mutex
	dir         string
	prefix      string
	loc         *time.Location
	retainDays  int
	currentDate string
	currentFile *os.File
}

func newRotatingWriter(dir, prefix string, retainDays int, loc *time.Location) (*rotatingWriter, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	w := &rotatingWriter{dir: dir, prefix: prefix, retainDays: retainDays, loc: loc}
	if err := w.rotate(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.rotateLocked(); err != nil {
		return 0, err
	}
	return w.currentFile.Write(p)
}

func (w *rotatingWriter) rotate() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.rotateLocked()
}

func (w *rotatingWriter) rotateLocked() error {
	today := time.Now().In(w.loc).Format("2006-01-02")
	if today == w.currentDate && w.currentFile != nil {
		return nil
	}
	if w.currentFile != nil {
		_ = w.currentFile.Close()
	}
	path := filepath.Join(w.dir, fmt.Sprintf("%s-%s.log", w.prefix, today))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	w.currentFile = f
	w.currentDate = today
	w.purgeOldLocked()
	return nil
}

func (w *rotatingWriter) purgeOldLocked() {
	cutoff := time.Now().In(w.loc).AddDate(0, 0, -w.retainDays)
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, w.prefix+"-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		dateStr := strings.TrimSuffix(strings.TrimPrefix(name, w.prefix+"-"), ".log")
		t, err := time.ParseInLocation("2006-01-02", dateStr, w.loc)
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			_ = os.Remove(filepath.Join(w.dir, name))
		}
	}
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.currentFile != nil {
		return w.currentFile.Close()
	}
	return nil
}

type loggingHandles struct {
	appOut    io.Writer
	accessOut io.Writer
	close     func() error
}

func setupLogging(logDir string, retainDays int) (*loggingHandles, error) {
	loc, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		loc = time.UTC
	}
	appW, err := newRotatingWriter(logDir, "app", retainDays, loc)
	if err != nil {
		return nil, err
	}
	accessW, err := newRotatingWriter(logDir, "access", retainDays, loc)
	if err != nil {
		_ = appW.Close()
		return nil, err
	}

	appOut := io.MultiWriter(os.Stdout, appW)
	accessOut := io.MultiWriter(os.Stdout, accessW)

	log.SetOutput(appOut)
	log.SetFlags(log.Ldate | log.Ltime)
	log.SetPrefix("")

	return &loggingHandles{
		appOut:    appOut,
		accessOut: accessOut,
		close: func() error {
			_ = appW.Close()
			return accessW.Close()
		},
	}, nil
}
