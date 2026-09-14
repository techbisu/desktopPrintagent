//go:build windows

package main

import (
	"fmt"
	"sync"

	"github.com/lxn/walk"

	"smartprint-agent/internal/queue"
)

// jobTableModel adapts a slice of queue.PrintJob to walk's TableView. New
// jobs are inserted at the top so the most recent activity is always
// visible without scrolling.
type jobTableModel struct {
	walk.TableModelBase
	mu   sync.Mutex
	jobs []queue.PrintJob
}

func newJobTableModel() *jobTableModel {
	return &jobTableModel{}
}

func (m *jobTableModel) RowCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.jobs)
}

func (m *jobTableModel) Value(row, col int) interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	if row < 0 || row >= len(m.jobs) {
		return ""
	}
	j := m.jobs[row]
	switch col {
	case 0:
		return j.Filename
	case 1:
		return fmt.Sprintf("%d x %d", j.Pages, j.Copies)
	case 2:
		if j.IsColor {
			return "Color"
		}
		return "B/W"
	case 3:
		return fmt.Sprintf("%.2f", j.TotalAmount)
	case 4:
		if j.PrinterAssigned == "" {
			return "-"
		}
		return j.PrinterAssigned
	case 5:
		return statusLabel(j)
	}
	return ""
}

// statusLabel renders a human-readable status, distinguishing jobs that
// are waiting on shopkeeper confirmation from ordinary queued jobs.
func statusLabel(j queue.PrintJob) string {
	if j.Status == queue.StatusQueued && j.PendingConfirmation {
		return "Awaiting confirmation"
	}
	switch j.Status {
	case queue.StatusQueued:
		return "Queued"
	case queue.StatusDownloading:
		return "Downloading"
	case queue.StatusPrinting:
		return "Printing"
	case queue.StatusPrinterQueued:
		return "Queued in printer"
	case queue.StatusPrinterOffline:
		return "Printer offline"
	case queue.StatusCompleted:
		return "Completed"
	case queue.StatusFailed:
		return "Failed"
	default:
		return string(j.Status)
	}
}

// SetJobs replaces the full job list (used for the initial snapshot load)
// and refreshes the view.
func (m *jobTableModel) SetJobs(jobs []queue.PrintJob) {
	// Show newest first.
	reversed := make([]queue.PrintJob, len(jobs))
	for i, j := range jobs {
		reversed[len(jobs)-1-i] = j
	}

	m.mu.Lock()
	m.jobs = reversed
	m.mu.Unlock()
	m.PublishRowsReset()
}

// Upsert inserts a new job at the top, or updates it in place if it's
// already known, then refreshes the view.
func (m *jobTableModel) Upsert(job queue.PrintJob) {
	m.mu.Lock()
	idx := -1
	for i, j := range m.jobs {
		if j.ID == job.ID {
			idx = i
			break
		}
	}
	if idx == -1 {
		m.jobs = append([]queue.PrintJob{job}, m.jobs...)
	} else {
		m.jobs[idx] = job
	}
	m.mu.Unlock()
	m.PublishRowsReset()
}

// JobAt returns the job at the given row, typically the TableView's
// currently selected index.
func (m *jobTableModel) JobAt(row int) (queue.PrintJob, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if row < 0 || row >= len(m.jobs) {
		return queue.PrintJob{}, false
	}
	return m.jobs[row], true
}
