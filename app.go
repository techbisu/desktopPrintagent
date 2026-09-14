package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"

	"smartprint-agent/internal/config"
	"smartprint-agent/internal/docprocessor"
	"smartprint-agent/internal/hardware"
	"smartprint-agent/internal/printer"
	"smartprint-agent/internal/queue"
	"smartprint-agent/internal/realtime"
)

// App holds all business logic: settings, the print pipeline, and the
// realtime connection. It knows nothing about whichever GUI toolkit is
// presenting it — main.go wires the UI's callbacks into these methods.
type App struct {
	cfgStore *config.Store
	engine   *printer.Engine
	manager  *queue.Manager
	pusher   *realtime.Client
}

// NewApp constructs the App with its dependencies wired together.
func NewApp() *App {
	cfgStore, err := config.NewStore()
	if err != nil {
		log.Fatalf("failed to initialize config store: %v", err)
	}

	engine := printer.NewEngine()
	processor := docprocessor.NewDirectPdfProcessor()
	manager := queue.NewManager(cfgStore, engine, processor)

	return &App{
		cfgStore: cfgStore,
		engine:   engine,
		manager:  manager,
	}
}

// Start extracts the embedded printer binary, launches the print worker
// pool, and connects to Pusher if the shop has already been configured.
// Call once at program startup, before running the UI's message loop.
func (a *App) Start() {
	if err := a.engine.Ensure(); err != nil {
		log.Printf("printer engine init failed: %v", err)
	}

	a.manager.Start(2) // 2 concurrent print workers is plenty for a single-shop counter.

	if a.cfgStore.IsConfigured() {
		a.startRealtime()
	}
}

// Stop tears down the Pusher connection cleanly. Call on window close.
func (a *App) Stop() {
	if a.pusher != nil {
		a.pusher.Stop()
	}
}

// startRealtime (re)creates the Pusher client from current config and
// starts it. Safe to call multiple times; the previous client is stopped
// first.
func (a *App) startRealtime() {
	if a.pusher != nil {
		a.pusher.Stop()
	}

	cfg := a.cfgStore.Get()
	if cfg.PusherAppKey == "" || cfg.PusherCluster == "" || cfg.PusherAuthURL == "" {
		log.Printf("realtime not started: Pusher settings incomplete")
		return
	}

	a.pusher = &realtime.Client{
		AppKey:      cfg.PusherAppKey,
		Cluster:     cfg.PusherCluster,
		ChannelName: fmt.Sprintf("private-shop-%s", cfg.ShopID),
		AuthURL:     cfg.PusherAuthURL,
		AuthToken:   cfg.AuthToken,
		OnJob:       a.handlePrintJobEvent,
	}
	a.pusher.Start()
}

// incomingJobPayload mirrors the JSON the web platform publishes on
// "new-print-job".
type incomingJobPayload struct {
	ID            string  `json:"id"`
	ServiceCode   string  `json:"serviceCode"`
	Filename      string  `json:"filename"`
	FileURL       string  `json:"fileUrl"`
	FileType      string  `json:"fileType"`
	Pages         int     `json:"pages"`
	Copies        int     `json:"copies"`
	IsColor       bool    `json:"isColor"`
	IsDuplex      bool    `json:"isDuplex"`
	TotalAmount   float64 `json:"totalAmount"`
	PaymentMethod string  `json:"paymentMethod"`
}

func (a *App) handlePrintJobEvent(dataJSON string) {
	var payload incomingJobPayload
	if err := json.Unmarshal([]byte(dataJSON), &payload); err != nil {
		log.Printf("failed to parse new-print-job payload: %v", err)
		return
	}

	id := payload.ID
	if id == "" {
		id = uuid.NewString()
	}

	job := queue.PrintJob{
		ID:            id,
		ServiceCode:   payload.ServiceCode,
		Filename:      payload.Filename,
		FileURL:       payload.FileURL,
		FileType:      payload.FileType,
		Pages:         payload.Pages,
		Copies:        payload.Copies,
		IsColor:       payload.IsColor,
		IsDuplex:      payload.IsDuplex,
		TotalAmount:   payload.TotalAmount,
		PaymentMethod: strings.ToLower(payload.PaymentMethod),
	}

	// UPI payments have no server-side payment webhook in the customer
	// portal. Hold those jobs even when auto-print is enabled so the operator
	// can verify the incoming UPI payment before releasing the document.
	if job.PaymentMethod == "upi" {
		a.manager.HoldForConfirmation(job)
		return
	}

	cfg := a.cfgStore.Get()
	if cfg.SilentAutoPrint {
		a.manager.Enqueue(job)
		return
	}

	// Manual confirmation mode: surface the job in the Live Queue tab as
	// QUEUED + pending, but withhold it from the print pipeline until the
	// shopkeeper clicks "Print now" (ConfirmJob).
	a.manager.HoldForConfirmation(job)
}

// ---- Methods below are called directly by the UI layer (ui.go) ----

// GetConfig returns the current settings for the Settings tab.
func (a *App) GetConfig() config.Config {
	return a.cfgStore.Get()
}

// SaveConfig persists new settings and restarts the realtime connection so
// changes to shop credentials or Pusher settings take effect immediately.
func (a *App) SaveConfig(cfg config.Config) error {
	if err := a.cfgStore.Save(cfg); err != nil {
		return err
	}
	a.startRealtime()
	return nil
}

// GetPrinters returns installed Windows printer names for the two dropdowns.
func (a *App) GetPrinters() ([]string, error) {
	return hardware.ListPrinters()
}

// GetQueue returns the current snapshot of all known jobs for the Live
// Queue tab's initial render (subsequent updates arrive via
// OnStatusChange).
func (a *App) GetQueue() []queue.PrintJob {
	return a.manager.Snapshot()
}

// OnStatusChange registers a callback invoked on every job status change.
func (a *App) OnStatusChange(fn queue.StatusListener) {
	a.manager.OnStatusChange(fn)
}

// RetryJob re-enqueues a failed job by ID.
func (a *App) RetryJob(jobID string) error {
	return a.manager.Retry(jobID)
}

// ConfirmJob releases a job that's awaiting manual confirmation (Silent
// Auto-Print off) into the print pipeline.
func (a *App) ConfirmJob(jobID string) error {
	return a.manager.Confirm(jobID)
}

// PreviewJob opens a selected queued or historical document for review.
func (a *App) PreviewJob(jobID string) error {
	return a.manager.Preview(jobID)
}
