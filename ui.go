//go:build windows

package main

import (
	"fmt"
	"log"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"

	"smartprint-agent/internal/config"
	"smartprint-agent/internal/queue"
)

// runUI builds and shows the main window, wires every widget to the App's
// methods, and blocks in walk's message loop until the window is closed
// for real (Quit from the tray, not just hidden).
func runUI(app *App) error {
	var mw *walk.MainWindow
	var queueTable *walk.TableView
	var retryBtn, confirmBtn, previewBtn *walk.PushButton

	var shopIDEdit, authTokenEdit *walk.LineEdit
	var pusherKeyEdit, pusherClusterEdit, pusherAuthURLEdit *walk.LineEdit
	var bwPrinterBox, colorPrinterBox *walk.ComboBox
	var autoPrintCheck *walk.CheckBox
	var saveStatusLabel, printerStatusLabel, previewStatusLabel *walk.Label

	model := newJobTableModel()

	quitting := false

	err := MainWindow{
		AssignTo: &mw,
		Title:    "SmartPrint Agent",
		Visible:  true,
		MinSize:  Size{Width: 600, Height: 400},
		Size:     Size{Width: 900, Height: 550},
		Layout:   VBox{MarginsZero: true, SpacingZero: true},
		Children: []Widget{
			Composite{
				MinSize: Size{Height: 66},
				MaxSize: Size{Height: 66},
				Layout:  VBox{Margins: Margins{Left: 22, Top: 10, Right: 22, Bottom: 8}, Spacing: 1},
				Children: []Widget{
					Label{Text: "SmartPrint Agent", Font: Font{Family: "Segoe UI", PointSize: 15, Bold: true}},
					Label{Text: "Secure local printing • Review payment requests and manage your print queue", Font: Font{Family: "Segoe UI", PointSize: 9}},
				},
			},
			TabWidget{
				Font: Font{Family: "Segoe UI", PointSize: 9},
				Pages: []TabPage{
					{
						Title:  "Live Queue",
						Layout: VBox{Margins: Margins{Left: 18, Top: 16, Right: 18, Bottom: 16}, Spacing: 10},
						Children: []Widget{
							Label{Text: "Print activity", Font: Font{Family: "Segoe UI", PointSize: 11, Bold: true}},
							Label{Text: "Preview documents, verify UPI payments, then release or retry jobs from one place."},
							TableView{
								AssignTo:         &queueTable,
								AlternatingRowBG: true,
								StretchFactor:    1,
								Columns: []TableViewColumn{
									{Title: "Document", Width: 220},
									{Title: "Pages / Copies", Width: 105},
									{Title: "Mode", Width: 70},
									{Title: "Amount", Width: 80},
									{Title: "Printer", Width: 180},
									{Title: "Status", Width: 160},
								},
								Model: model,
							},
							Composite{
								Layout: HBox{},
								Children: []Widget{
									Label{AssignTo: &previewStatusLabel, Text: "Select a job to take action."},
									HSpacer{},
									PushButton{
										AssignTo: &previewBtn,
										Text:     "Preview document",
										OnClicked: func() {
											handlePreview(app, queueTable, model, mw, previewStatusLabel, previewBtn)
										},
									},
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
						Layout: VBox{Margins: Margins{Left: 18, Top: 16, Right: 18, Bottom: 16}, Spacing: 10},
						Children: []Widget{
							Label{Text: "Agent setup", Font: Font{Family: "Segoe UI", PointSize: 11, Bold: true}},
							Label{Text: "Enter your shop credentials, then choose the printers this workstation should use."},
							GroupBox{
								Title:  "1. Shop credentials",
								Layout: Grid{Columns: 2, Margins: Margins{Left: 14, Top: 16, Right: 14, Bottom: 12}, Spacing: 8},
								Children: []Widget{
									Label{Text: "Shop ID", MinSize: Size{Width: 130}},
									LineEdit{AssignTo: &shopIDEdit},
									Label{Text: "Access token"},
									LineEdit{AssignTo: &authTokenEdit, PasswordMode: true},
								},
							},
							GroupBox{
								Title:  "2. Realtime connection",
								Layout: Grid{Columns: 2, Margins: Margins{Left: 14, Top: 16, Right: 14, Bottom: 12}, Spacing: 8},
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
								Title:  "3. Printer routing",
								Layout: Grid{Columns: 2, Margins: Margins{Left: 14, Top: 16, Right: 14, Bottom: 12}, Spacing: 8},
								Children: []Widget{
									Label{Text: "Black & white printer"},
									ComboBox{AssignTo: &bwPrinterBox, Editable: false, MinSize: Size{Width: 360, Height: 26}},
									Label{Text: "Color printer"},
									ComboBox{AssignTo: &colorPrinterBox, Editable: false, MinSize: Size{Width: 360, Height: 26}},
								},
							},
							Composite{
								Layout: HBox{},
								Children: []Widget{
									PushButton{
										Text: "Refresh printers",
										OnClicked: func() {
											loadPrintersIntoBoxes(app, bwPrinterBox, colorPrinterBox, printerStatusLabel)
										},
									},
									Label{AssignTo: &printerStatusLabel, Text: ""},
									HSpacer{},
								},
							},
							GroupBox{
								Title:  "4. Print behavior",
								Layout: VBox{Margins: Margins{Left: 14, Top: 14, Right: 14, Bottom: 12}, Spacing: 4},
								Children: []Widget{
									CheckBox{AssignTo: &autoPrintCheck, Text: "Automatically print jobs as soon as they arrive"},
									Label{Text: "Turn this off to review each job in the Live Queue before printing."},
								},
							},
							Composite{
								Layout: HBox{},
								Children: []Widget{
									PushButton{
										Text: "Save configuration",
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

	// Center the window on the primary screen
	if hwnd := mw.Handle(); hwnd != 0 {
		cxScreen := win.GetSystemMetrics(win.SM_CXSCREEN)
		cyScreen := win.GetSystemMetrics(win.SM_CYSCREEN)
		x := (cxScreen - int32(mw.Width())) / 2
		y := (cyScreen - int32(mw.Height())) / 2
		
		mw.SetBounds(walk.Rectangle{
			X:      int(x),
			Y:      int(y),
			Width:  mw.Width(),
			Height: mw.Height(),
		})
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
			if job.Status == queue.StatusQueued && job.PendingConfirmation && job.PaymentMethod == "upi" {
				promptUPIPaymentConfirmation(app, job)
			}
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
	// A walk ComboBox remains visually blank until it has a selected index,
	// even when its model contains items. Choose a usable default so the
	// discovered printers are immediately visible.
	bwPrinterBox.SetCurrentIndex(0)
	colorPrinterBox.SetCurrentIndex(0)
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

func handlePreview(app *App, table *walk.TableView, model *jobTableModel, mw *walk.MainWindow, statusLabel *walk.Label, previewBtn *walk.PushButton) {
	idx := table.CurrentIndex()
	job, ok := model.JobAt(idx)
	if !ok {
		statusLabel.SetText("Select a job before previewing.")
		return
	}
	statusLabel.SetText("Preparing secure preview…")
	previewBtn.SetEnabled(false)
	go func() {
		err := app.PreviewJob(job.ID)
		mw.Synchronize(func() {
			previewBtn.SetEnabled(true)
			if err != nil {
				statusLabel.SetText(fmt.Sprintf("Preview failed: %v", err))
				return
			}
			statusLabel.SetText("Preview opened. The temporary file is removed when you close it.")
		})
	}()
}

func promptUPIPaymentConfirmation(app *App, job queue.PrintJob) {
	message := fmt.Sprintf(
		"UPI payment verification is required before printing.\n\nDocument: %s\nAmount: INR %.2f\n\nConfirm that payment has arrived in the shop's UPI app, then choose Yes to print.",
		job.Filename, job.TotalAmount,
	)
	if walk.MsgBox(nil, "Confirm UPI payment", message, walk.MsgBoxYesNo|walk.MsgBoxIconInformation) == walk.DlgCmdYes {
		if err := app.ConfirmJob(job.ID); err != nil {
			log.Printf("UPI print confirmation failed: %v", err)
		}
	}
}
