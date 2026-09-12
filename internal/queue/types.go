package queue

// Status represents where a job is in its lifecycle. The frontend Queue tab
// listens for these exact string values on the "job:status" Wails event.
type Status string

const (
	StatusQueued      Status = "QUEUED"
	StatusDownloading Status = "DOWNLOADING"
	StatusPrinting    Status = "PRINTING"
	StatusCompleted   Status = "COMPLETED"
	StatusFailed      Status = "FAILED"
)

// PrintJob is the full record tracked by the desktop agent for a single
// print job, from cloud event through to shredding.
type PrintJob struct {
	ID              string  `json:"id"`
	ServiceCode     string  `json:"serviceCode"`
	Filename        string  `json:"filename"`
	FileURL         string  `json:"fileUrl"`
	FileType        string  `json:"fileType"`
	Pages           int     `json:"pages"`
	Copies          int     `json:"copies"`
	IsColor         bool    `json:"isColor"`
	IsDuplex        bool    `json:"isDuplex"`
	TotalAmount     float64 `json:"totalAmount"`
	Status          Status  `json:"status"`
	PrinterAssigned string  `json:"printerAssigned"`
	Error           string  `json:"error,omitempty"`
	// PendingConfirmation is true when Silent Auto-Print is off and this
	// job is sitting in the queue waiting for the shopkeeper to tap
	// "Print Now" in the Live Queue tab.
	PendingConfirmation bool `json:"pendingConfirmation,omitempty"`
}
