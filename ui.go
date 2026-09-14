//go:build windows

package main

import (
	"fmt"
	"log"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"smartprint-agent/internal/config"
	"smartprint-agent/internal/queue"
)

// runUI builds and shows the main window, wires every widget to the App's
// methods, and blocks in walk's message loop until the window is closed
// for real (Quit from the tray, not just hidden).
func runUI(app *App) error {
	var mw *walk.MainWindow
	var queueTable *walk.TableView
	var retryBtn, confirmBtn *walk.PushButton

	var shopIDEdit, authTokenEdit *walk.LineEdit
	var pusherKeyEdit, pusherClusterEdit, pusherAuthURLEdit *walk.LineEdit
	var bwPrinterBox, colorPrinterBox *walk.ComboBox
	var autoPrintCheck *walk.CheckBox
	var saveStatusLabel, printerStatusLabel *walk.Label

	model := newJobTableModel()

	quitting := false

	err := MainWindow{
		AssignTo: &mw,
		Title:    "SmartPrint Agent",
		Visible:  true,
		MinSize:  Size{Width: 640, Height: 460},
		Size:     Size{Width: 720, Height: 500},
		Layout:   VBox{},
		Children: []Widget{
			TabWidget{
				Pages: []TabPage{
					{
						Title:  "Live queue",
						Layout: VBox{},
						Children: []Widget{
							TableView{
								AssignTo:         &queueTable,
								AlternatingRowBG: true,
								Columns: []TableViewColumn{
									{Title: "Filename", Width: 180},
									{Title: "Pages x copies", Width: 100},
									{Title: "Type", Width: 60},
									{Title: "Price", Width: 70},
									{Title: "Printer", Width: 130},
									{Title: "Status", Width: 150},
								},
								Model: model,
							},
							Composite{
								Layout: HBox{},
								Children: []Widget{
									HSpacer{},
									PushButton{
										AssignTo: &confirmBtn,
										Text:     "Print now",
										OnClicked: func() {
											handleConfirm(app, queueTable, model)
										},
									},
									PushButton{
										AssignTo: &retryBtn,
										Text:     "Retry",
										OnClicked: func() {
											handleRetry(app, queueTable, model)
										},
									},
								},
							},
						},
					},
					{
						Title:  "Settings",
						Layout: VBox{},
						Children: []Widget{
							GroupBox{
								Title:  "Shop credentials",
								Layout: Grid{Columns: 2},
								Children: []Widget{
									Label{Text: "Shop ID"},
									LineEdit{AssignTo: &shopIDEdit},
									Label{Text: "Auth token"},
									LineEdit{AssignTo: &authTokenEdit, PasswordMode: true},
								},
							},
							GroupBox{
								Title:  "Realtime connection",
								Layout: Grid{Columns: 2},
								Children: []Widget{
									Label{Text: "Pusher app key"},
									LineEdit{AssignTo: &pusherKeyEdit},
									Label{Text: "Pusher cluster"},
									LineEdit{AssignTo: &pusherClusterEdit},
									Label{Text: "Auth endpoint URL"},
									LineEdit{AssignTo: &pusherAuthURLEdit},
								},
							},
							GroupBox{
								Title:  "Printers",
								Layout: Grid{Columns: 2},
								Children: []Widget{
									Label{Text: "Black and white printer"},
									ComboBox{AssignTo: &bwPrinterBox, Editable: false},
									Label{Text: "Color printer"},
									ComboBox{AssignTo: &colorPrinterBox, Editable: false},
								},
							},
							Composite{
								Layout: HBox{},
								Children: []Widget{
									PushButton{
										Text: "Refresh printer list",
										OnClicked: func() {
											loadPrintersIntoBoxes(app, bwPrinterBox, colorPrinterBox, printerStatusLabel)
										},
									},
									Label{AssignTo: &printerStatusLabel, Text: ""},
									HSpacer{},
								},
							},
							CheckBox{
								AssignTo: &autoPrintCheck,
								Text:     "Silent auto-print (print jobs the instant they arrive)",
							},
							Composite{
								Layout: HBox{},
								Children: []Widget{
									PushButton{
										Text: "Save settings",
										OnClicked: func() {
											saveSettings(app, shopIDEdit, authTokenEdit,
												pusherKeyEdit, pusherClusterEdit, pusherAuthURLEdit,
												bwPrinterBox, colorPrinterBox, autoPrintCheck, saveStatusLabel)
										},
									},
									Label{AssignTo: &saveStatusLabel, Text: ""},
									HSpacer{},
								},
							},
						},
					},
				},
			},
		},
	}.Create()
	if err != nil {
		return err
	}

	// Minimize to tray instead of exiting, unless Quit was chosen from the
	// tray menu.
	mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		if quitting {
			return
		}
		*canceled = true
		mw.Hide()
	})

	if _, err := setupTray(mw, &quitting); err != nil {
		log.Printf("tray setup failed (continuing without tray icon): %v", err)
	}

	loadSettingsIntoForm(app, shopIDEdit, authTokenEdit, pusherKeyEdit, pusherClusterEdit,
		pusherAuthURLEdit, bwPrinterBox, colorPrinterBox, autoPrintCheck, printerStatusLabel)

	model.SetJobs(app.GetQueue())

	// Every status change from the print pipeline arrives on a worker
	// goroutine. walk widgets may only be touched from the UI thread, so
	// marshal the update via Synchronize.
	app.OnStatusChange(func(job queue.PrintJob) {
		mw.Synchronize(func() {
			model.Upsert(job)
		})
	})

	mw.Run()
	return nil
}

func loadSettingsIntoForm(
	app *App,
	shopIDEdit, authTokenEdit, pusherKeyEdit, pusherClusterEdit, pusherAuthURLEdit *walk.LineEdit,
	bwPrinterBox, colorPrinterBox *walk.ComboBox,
	autoPrintCheck *walk.CheckBox,
	printerStatusLabel *walk.Label,
) {
	cfg := app.GetConfig()
	shopIDEdit.SetText(cfg.ShopID)
	authTokenEdit.SetText(cfg.AuthToken)
	pusherKeyEdit.SetText(cfg.PusherAppKey)
	pusherClusterEdit.SetText(cfg.PusherCluster)
	pusherAuthURLEdit.SetText(cfg.PusherAuthURL)
	autoPrintCheck.SetChecked(cfg.SilentAutoPrint)

	loadPrintersIntoBoxes(app, bwPrinterBox, colorPrinterBox, printerStatusLabel)
	printers := printerNames(bwPrinterBox)
	selectComboValue(bwPrinterBox, printers, cfg.BlackWhitePrinter)
	selectComboValue(colorPrinterBox, printers, cfg.ColorPrinter)
}

func loadPrintersIntoBoxes(app *App, bwPrinterBox, colorPrinterBox *walk.ComboBox, statusLabel *walk.Label) {
	printers, err := app.GetPrinters()
	if err != nil {
		log.Printf("failed to list printers: %v", err)
		statusLabel.SetText("Could not load printers; see agent.log")
		return
	}
	bwPrinterBox.SetModel(printers)
	colorPrinterBox.SetModel(printers)
	if len(printers) == 0 {
		statusLabel.SetText("No installed printers detected")
		return
	}
	statusLabel.SetText(fmt.Sprintf("Found %d printer(s)", len(printers)))
}

func printerNames(box *walk.ComboBox) []string {
	printers, _ := box.Model().([]string)
	return printers
}

func selectComboValue(box *walk.ComboBox, options []string, value string) {
	for i, opt := range options {
		if opt == value {
			box.SetCurrentIndex(i)
			return
		}
	}
}

func saveSettings(
	app *App,
	shopIDEdit, authTokenEdit, pusherKeyEdit, pusherClusterEdit, pusherAuthURLEdit *walk.LineEdit,
	bwPrinterBox, colorPrinterBox *walk.ComboBox,
	autoPrintCheck *walk.CheckBox,
	statusLabel *walk.Label,
) {
	cfg := config.Config{
		ShopID:            shopIDEdit.Text(),
		AuthToken:         authTokenEdit.Text(),
		PusherAppKey:      pusherKeyEdit.Text(),
		PusherCluster:     pusherClusterEdit.Text(),
		PusherAuthURL:     pusherAuthURLEdit.Text(),
		BlackWhitePrinter: comboText(bwPrinterBox),
		ColorPrinter:      comboText(colorPrinterBox),
		SilentAutoPrint:   autoPrintCheck.Checked(),
	}

	if err := app.SaveConfig(cfg); err != nil {
		statusLabel.SetText(fmt.Sprintf("Error: %v", err))
		return
	}
	statusLabel.SetText("Saved.")
}

func comboText(box *walk.ComboBox) string {
	idx := box.CurrentIndex()
	model, ok := box.Model().([]string)
	if !ok || idx < 0 || idx >= len(model) {
		return ""
	}
	return model[idx]
}

func handleRetry(app *App, table *walk.TableView, model *jobTableModel) {
	idx := table.CurrentIndex()
	job, ok := model.JobAt(idx)
	if !ok {
		return
	}
	if err := app.RetryJob(job.ID); err != nil {
		log.Printf("retry failed: %v", err)
	}
}

func handleConfirm(app *App, table *walk.TableView, model *jobTableModel) {
	idx := table.CurrentIndex()
	job, ok := model.JobAt(idx)
	if !ok {
		return
	}
	if err := app.ConfirmJob(job.ID); err != nil {
		log.Printf("confirm failed: %v", err)
	}
}
