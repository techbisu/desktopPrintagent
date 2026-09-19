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
		if j.ServiceCode != "" {
			return fmt.Sprintf("[%s] %s", j.ServiceCode, j.Filename)
		}
		return j.Filename
	case 1:
		return fmt.Sprintf("%d pgs × %d", j.Pages, j.Copies)
	case 2:
		if j.IsColor {
			return "🎨 Color"
		}
		return "🔲 B/W"
	case 3:
		return fmt.Sprintf("₹ %.2f", j.TotalAmount)
	case 4:
		if j.PrinterAssigned == "" {
			return "—"
		}
		return j.PrinterAssigned
	case 5:
		return statusLabel(j)
	}
	return ""
}

// StyleCell applies distinct colors to status states in the table.
func (m *jobTableModel) StyleCell(style *walk.CellStyle) {
	row := style.Row()
	if row < 0 {
		return
	}
	m.mu.Lock()
	if row >= len(m.jobs) {
		m.mu.Unlock()
		return
	}
	j := m.jobs[row]
	m.mu.Unlock()

	if style.Col() == 5 {
		switch {
		case j.Status == queue.StatusQueued && j.PendingConfirmation:
			style.TextColor = walk.RGB(190, 85, 0)
		case j.Status == queue.StatusFailed || j.Status == queue.StatusPrinterOffline:
			style.TextColor = walk.RGB(200, 20, 20)
		case j.Status == queue.StatusPrinting || j.Status == queue.StatusDownloading:
			style.TextColor = walk.RGB(0, 102, 204)
		case j.Status == queue.StatusCompleted:
			style.TextColor = walk.RGB(16, 130, 48)
		}
	}
}

// statusLabel renders a human-readable status, distinguishing jobs that
// are waiting on shopkeeper confirmation from ordinary queued jobs.
func statusLabel(j queue.PrintJob) string {
	if j.Status == queue.StatusQueued && j.PendingConfirmation {
		if j.PaymentMethod == "upi" {
			return "⚠️ Awaiting UPI Confirmation"
		}
		return "⏳ Needs Manual Approval"
	}
	switch j.Status {
	case queue.StatusQueued:
		return "⏳ In Queue"
	case queue.StatusDownloading:
		return "📥 Downloading..."
	case queue.StatusPrinting:
		return "🖨️ Printing..."
	case queue.StatusPrinterQueued:
		return "📄 Sent to printer"
	case queue.StatusPrinterOffline:
		return "⚠️ Printer offline"
	case queue.StatusCompleted:
		return "✅ Completed"
	case queue.StatusFailed:
		if j.Error != "" {
			return fmt.Sprintf("❌ Failed (%s)", j.Error)
		}
		return "❌ Failed"
	default:
		return string(j.Status)
	}
}

// GetStats returns counts for active, pending, completed, and failed jobs.
func (m *jobTableModel) GetStats() (total, pending, active, completed, failed int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	total = len(m.jobs)
	for _, j := range m.jobs {
		if j.Status == queue.StatusQueued && j.PendingConfirmation {
			pending++
		} else if j.Status == queue.StatusPrinting || j.Status == queue.StatusDownloading || j.Status == queue.StatusPrinterQueued {
			active++
		} else if j.Status == queue.StatusCompleted {
			completed++
		} else if j.Status == queue.StatusFailed || j.Status == queue.StatusPrinterOffline {
			failed++
		}
	}
	return
}

// FirstPendingJob returns the earliest job needing operator action, if any.
func (m *jobTableModel) FirstPendingJob() (queue.PrintJob, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.jobs {
		if j.Status == queue.StatusQueued && j.PendingConfirmation {
			return j, true
		}
	}
	return queue.PrintJob{}, false
}

// ClearCompleted removes completed jobs from the table.
func (m *jobTableModel) ClearCompleted() {
	m.mu.Lock()
	filtered := make([]queue.PrintJob, 0, len(m.jobs))
	for _, j := range m.jobs {
		if j.Status != queue.StatusCompleted {
			filtered = filtered
			filtered = append(filtered, j)
		}
	}
	m.jobs = filtered
	m.mu.Unlock()
	m.PublishRowsReset()
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
