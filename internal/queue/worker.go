// Package queue implements the print job pipeline: a buffered channel feeds
// a small pool of workers that download the file, hand it to a
// DocumentProcessor, print it, and unconditionally shred the temp file
// afterward regardless of outcome.
package queue

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"smartprint-agent/internal/config"
	"smartprint-agent/internal/docprocessor"
	"smartprint-agent/internal/hardware"
	"smartprint-agent/internal/printer"
)

// bufferSize bounds how many jobs can be pending before Enqueue blocks the
// caller (the Pusher event handler), preventing unbounded memory growth if
// jobs arrive faster than they can be printed.
const bufferSize = 100

// StatusListener is called on every status transition for any job. The UI
// registers one of these via OnStatusChange to keep the Live Queue view in
// sync. Manager calls listeners synchronously from whichever goroutine
// changed the status, so a listener that touches UI widgets MUST marshal
// itself onto the UI thread (e.g. via walk.Window.Synchronize) before
// touching any widget — Manager makes no such guarantee itself.
type StatusListener func(PrintJob)

// Manager owns the job channel, worker goroutines, and in-memory history
// used to populate the Live Queue tab.
type Manager struct {
	cfgStore  *config.Store
	engine    *printer.Engine
	processor docprocessor.DocumentProcessor
	jobs      chan PrintJob
	quit      chan struct{}
	stopOnce  sync.Once

	mu        sync.Mutex
	history   map[string]*PrintJob
	order     []string // preserves insertion order for Snapshot
	listeners []StatusListener
}

// NewManager constructs a Manager. Call Start to launch its worker pool.
func NewManager(cfgStore *config.Store, engine *printer.Engine, processor docprocessor.DocumentProcessor) *Manager {
	return &Manager{
		cfgStore:  cfgStore,
		engine:    engine,
		processor: processor,
		jobs:      make(chan PrintJob, bufferSize),
		quit:      make(chan struct{}),
		history:   make(map[string]*PrintJob),
	}
}

// OnStatusChange registers a listener invoked on every job status
// transition. Safe to call before or after Start.
func (m *Manager) OnStatusChange(fn StatusListener) {
	m.mu.Lock()
	m.listeners = append(m.listeners, fn)
	m.mu.Unlock()
}

// Start launches workerCount goroutines that pull jobs off the channel and
// process them one at a time each.
func (m *Manager) Start(workerCount int) {
	if workerCount < 1 {
		workerCount = 1
	}
	for i := 0; i < workerCount; i++ {
		go m.workerLoop()
	}
}

// Stop signals all worker goroutines in the pool to shut down cleanly.
func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		close(m.quit)
	})
}

// Enqueue adds a new job straight into the print pipeline. Used for
// Silent Auto-Print jobs and for retries.
func (m *Manager) Enqueue(job PrintJob) {
	job.PendingConfirmation = false
	m.setStatus(&job, StatusQueued, "")
	m.jobs <- job
}

// HoldForConfirmation records a job as QUEUED and pending shopkeeper
// confirmation, but does not push it into the print pipeline. Used when
// Silent Auto-Print is turned off. Call Confirm once the shopkeeper clicks
// "Print now" in the Live Queue tab.
func (m *Manager) HoldForConfirmation(job PrintJob) {
	job.PendingConfirmation = true
	m.setStatus(&job, StatusQueued, "")
}

// Confirm releases a job that was held by HoldForConfirmation into the
// print pipeline.
func (m *Manager) Confirm(jobID string) error {
	m.mu.Lock()
	job, ok := m.history[jobID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("job %s not found", jobID)
	}
	if !job.PendingConfirmation {
		return fmt.Errorf("job %s is not awaiting confirmation", jobID)
	}
	confirmed := *job
	confirmed.PendingConfirmation = false
	m.setStatus(&confirmed, StatusQueued, "")
	m.jobs <- confirmed
	return nil
}

// Snapshot returns a copy of all known jobs in the order they were
// received, for the UI's initial render before live updates start
// arriving via OnStatusChange.
func (m *Manager) Snapshot() []PrintJob {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]PrintJob, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, *m.history[id])
	}
	return out
}

// Retry re-enqueues a previously failed job by ID.
func (m *Manager) Retry(jobID string) error {
	m.mu.Lock()
	job, ok := m.history[jobID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("job %s not found", jobID)
	}
	retryJob := *job
	retryJob.Error = ""
	m.Enqueue(retryJob)
	return nil
}

// Preview downloads a temporary copy of a job and opens it in SumatraPDF.
// Preview files are removed once the viewer process exits, preserving the
// agent's privacy guarantees without freezing the UI while the user reviews
// a document.
func (m *Manager) Preview(jobID string) error {
	m.mu.Lock()
	job, ok := m.history[jobID]
	if ok {
		copy := *job
		job = &copy
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("job %s not found", jobID)
	}
	if job.FileURL == "" {
		return fmt.Errorf("job %s has no document URL", jobID)
	}

	tempPath, err := m.downloadPreview(*job)
	if err != nil {
		return fmt.Errorf("download preview: %w", err)
	}
	cmd, err := m.engine.StartPreview(tempPath)
	if err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	go func() {
		_ = cmd.Wait()
		_ = os.Remove(tempPath)
	}()
	return nil
}

func (m *Manager) workerLoop() {
	for {
		select {
		case <-m.quit:
			return
		case job, ok := <-m.jobs:
			if !ok {
				return
			}
			m.process(job)
		}
	}
}

func (m *Manager) process(job PrintJob) {
	m.setStatus(&job, StatusDownloading, "")

	tempPath, err := m.download(job)
	if err != nil {
		m.setStatus(&job, StatusFailed, fmt.Sprintf("download failed: %v", err))
		return
	}
	// CRITICAL: the downloaded file is always shredded once this function
	// returns, whether the print succeeded or failed. Customer documents
	// (IDs, legal papers) must never persist on the shop's PC.
	defer os.Remove(tempPath)

	printPath, err := m.processor.Process(tempPath)
	if err != nil {
		m.setStatus(&job, StatusFailed, fmt.Sprintf("document processing failed: %v", err))
		return
	}
	// If the processor produced a distinct converted file, make sure that
	// gets shredded too.
	if printPath != tempPath {
		defer os.Remove(printPath)
	}

	cfg := m.cfgStore.Get()
	printerName := cfg.BlackWhitePrinter
	if job.IsColor {
		printerName = cfg.ColorPrinter
	}
	job.PrinterAssigned = printerName
	state, err := hardware.GetPrinterState(printerName)
	if err != nil {
		m.setStatus(&job, StatusFailed, fmt.Sprintf("could not verify printer %q: %v", printerName, err))
		return
	}
	if !state.Found {
		m.setStatus(&job, StatusPrinterOffline, fmt.Sprintf("printer %q is not installed", printerName))
		return
	}
	if state.Offline {
		m.setStatus(&job, StatusPrinterOffline, fmt.Sprintf("printer %q is offline (%s)", printerName, state.Status))
		return
	}

	m.setStatus(&job, StatusPrinting, "")

	err = m.engine.ExecutePrint(printer.Job{
		FilePath:    printPath,
		PrinterName: printerName,
		Copies:      job.Copies,
		Color:       job.IsColor,
		Duplex:      job.IsDuplex,
	})
	if err != nil {
		m.setStatus(&job, StatusFailed, err.Error())
		return
	}

	// SumatraPDF has handed the document to the Windows spooler. Windows may
	// still wait for paper, a network printer, or a device recovering from an
	// error, so do not claim the physical page was completed here.
	m.setStatus(&job, StatusPrinterQueued, "Accepted by the Windows print queue")
}

// download streams the job's file to %Temp%/SmartPrint/<job_id>.<ext>.
func (m *Manager) download(job PrintJob) (string, error) {
	dir := filepath.Join(os.TempDir(), "SmartPrint")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	ext := job.FileType
	if ext == "" {
		ext = "pdf"
	}
	destPath := filepath.Join(dir, fmt.Sprintf("%s.%s", job.ID, ext))

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(job.FileURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d fetching file", resp.StatusCode)
	}

	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", err
	}
	return destPath, nil
}

func (m *Manager) downloadPreview(job PrintJob) (string, error) {
	dir := filepath.Join(os.TempDir(), "SmartPrint")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(dir, "preview-*.pdf")
	if err != nil {
		return "", err
	}
	path := file.Name()
	defer func() {
		if file != nil {
			_ = file.Close()
		}
	}()

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(job.FileURL)
	if err != nil {
		_ = os.Remove(path)
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_ = os.Remove(path)
		return "", fmt.Errorf("unexpected status %d fetching file", resp.StatusCode)
	}
	if _, err := io.Copy(file, resp.Body); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	file = nil
	return path, nil
}

func (m *Manager) record(job *PrintJob) {
	m.mu.Lock()
	if _, exists := m.history[job.ID]; !exists {
		m.order = append(m.order, job.ID)
	}
	cp := *job
	m.history[job.ID] = &cp
	m.mu.Unlock()
}

func (m *Manager) setStatus(job *PrintJob, status Status, errMsg string) {
	job.Status = status
	job.Error = errMsg
	m.record(job)

	m.mu.Lock()
	listeners := make([]StatusListener, len(m.listeners))
	copy(listeners, m.listeners)
	m.mu.Unlock()

	for _, fn := range listeners {
		fn(*job)
	}
}
