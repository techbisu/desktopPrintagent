//go:build windows

package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"

	"smartprint-agent/internal/config"
	"smartprint-agent/internal/queue"
	"smartprint-agent/internal/realtime"
)

// runUI builds and shows the main window, wires every widget to the App's
// methods, and blocks in walk's message loop until the window is closed
// for real (Quit from the tray, not just hidden).
func runUI(app *App, startMinimized bool) error {
	var mw *walk.MainWindow
	var queueTable *walk.TableView
	var retryBtn, confirmBtn, previewBtn, clearBtn *walk.PushButton

	var logoImageView *walk.ImageView
	var headerConnLabel, headerStatsLabel *walk.Label
	var alertBanner, progressComposite *walk.Composite
	var alertIconLabel, alertTextLabel, progressLabel *walk.Label
	var alertActionBtn, alertDismissBtn *walk.PushButton
	var progressBar *walk.ProgressBar
	var selectionHintLabel *walk.Label

	var connStatusItem, queueStatsItem, printerInfoItem, autoPrintItem *walk.StatusBarItem

	var shopIDEdit, authTokenEdit *walk.LineEdit
	var pusherKeyEdit, pusherClusterEdit, pusherAuthURLEdit *walk.LineEdit
	var bwPrinterBox, colorPrinterBox *walk.ComboBox
	var autoPrintCheck, autoAcceptUPICheck, autoStartCheck *walk.CheckBox
	var saveStatusLabel, printerStatusLabel *walk.Label

	model := newJobTableModel()
	quitting := false

	// Helper to refresh dashboard status bar & badges
	updateDashboard := func() {
		if mw == nil {
			return
		}
		// Connection state label in header
		state := app.GetConnectionState()
		switch state {
		case realtime.StateConnected:
			if headerConnLabel != nil {
				headerConnLabel.SetText("● Connected (Live)")
			}
			if connStatusItem != nil {
				_ = connStatusItem.SetText("● Pusher: Connected")
			}
		case realtime.StateConnecting:
			if headerConnLabel != nil {
				headerConnLabel.SetText("○ Connecting...")
			}
			if connStatusItem != nil {
				_ = connStatusItem.SetText("○ Pusher: Connecting...")
			}
		default:
			if headerConnLabel != nil {
				headerConnLabel.SetText("✕ Offline")
			}
			if connStatusItem != nil {
				_ = connStatusItem.SetText("✕ Pusher: Disconnected")
			}
		}

		// Stats
		total, pending, active, completed, failed := model.GetStats()
		if headerStatsLabel != nil {
			headerStatsLabel.SetText(fmt.Sprintf("Total: %d  |  Pending: %d  |  Active: %d  |  Done: %d", total, pending, active, completed))
		}
		if queueStatsItem != nil {
			statsText := fmt.Sprintf("Queue: %d | Active: %d | Done: %d", total, active, completed)
			if pending > 0 {
				statsText = fmt.Sprintf("⚠️ %d Pending | %s", pending, statsText)
			}
			if failed > 0 {
				statsText = fmt.Sprintf("%s | ❌ %d Failed", statsText, failed)
			}
			_ = queueStatsItem.SetText(statsText)
		}

		// Printer info in status bar
		if printerInfoItem != nil {
			cfg := app.GetConfig()
			bw := cfg.BlackWhitePrinter
			if bw == "" {
				bw = "None"
			}
			col := cfg.ColorPrinter
			if col == "" {
				col = "None"
			}
			_ = printerInfoItem.SetText(fmt.Sprintf("Printers — B&W: %s | Color: %s", bw, col))
		}

		// Auto-Print status item
		if autoPrintItem != nil {
			cfg := app.GetConfig()
			modeStr := "Manual Confirmation"
			if cfg.SilentAutoPrint {
				if cfg.AutoAcceptUPI {
					modeStr = "Auto-Print (All + UPI)"
				} else {
					modeStr = "Auto-Print (UPI Verify)"
				}
			}
			_ = autoPrintItem.SetText("Mode: " + modeStr)
		}

		// Active progress indicator
		if progressComposite != nil && progressBar != nil && progressLabel != nil {
			if active > 0 {
				progressComposite.SetVisible(true)
				progressBar.SetVisible(true)
				_ = progressBar.SetMarqueeMode(true)
				progressLabel.SetText(fmt.Sprintf("🖨️ Processing %d active print job(s) in background...", active))
			} else {
				progressComposite.SetVisible(false)
			}
		}

		// In-window Alert Banner for jobs needing manual approval
		if alertBanner != nil && alertTextLabel != nil && alertActionBtn != nil {
			if pendingJob, ok := model.FirstPendingJob(); ok {
				alertBanner.SetVisible(true)
				if isManualPayment(pendingJob.PaymentMethod) {
					alertIconLabel.SetText("⚠️")
					alertTextLabel.SetText(fmt.Sprintf("Payment Verification Required: '%s' (₹%.2f) — Verify shopkeeper account, then click Approve", pendingJob.Filename, pendingJob.TotalAmount))
				} else {
					alertIconLabel.SetText("⏳")
					alertTextLabel.SetText(fmt.Sprintf("Manual Print Approval Required: '%s' (%d pgs × %d cpy)", pendingJob.Filename, pendingJob.Pages, pendingJob.Copies))
				}
				alertActionBtn.SetText("Approve & Print")
				alertActionBtn.SetVisible(true)
			} else if failed > 0 {
				alertBanner.SetVisible(true)
				alertIconLabel.SetText("❌")
				alertTextLabel.SetText(fmt.Sprintf("%d job(s) failed. Check printer paper, cables, and error details.", failed))
				alertActionBtn.SetText("Retry All")
				alertActionBtn.SetVisible(false)
			} else {
				alertBanner.SetVisible(false)
			}
		}
	}

	err := MainWindow{
		AssignTo: &mw,
		Title:    "SmartPrint Agent — Desktop Print Station",
		Visible:  !startMinimized,
		MinSize:  Size{Width: 720, Height: 480},
		Size:     Size{Width: 960, Height: 600},
		Layout:   VBox{MarginsZero: true, SpacingZero: true},
		StatusBarItems: []StatusBarItem{
			{AssignTo: &connStatusItem, Width: 180, ToolTipText: "Realtime WebSocket connection status"},
			{AssignTo: &queueStatsItem, Width: 260, ToolTipText: "Active jobs and queue counters"},
			{AssignTo: &printerInfoItem, Width: 320, ToolTipText: "Configured hardware printers"},
			{AssignTo: &autoPrintItem, Width: 180, ToolTipText: "Silent Auto-Print operational mode"},
		},
		Children: []Widget{
			// Top Banner Header
			Composite{
				MinSize: Size{Height: 64},
				MaxSize: Size{Height: 64},
				Layout:  HBox{Margins: Margins{Left: 20, Top: 10, Right: 20, Bottom: 10}, Spacing: 12},
				Children: []Widget{
					ImageView{
						AssignTo: &logoImageView,
						Mode:     ImageViewModeShrink,
						MinSize:  Size{Width: 44, Height: 44},
						MaxSize:  Size{Width: 44, Height: 44},
					},
					Composite{
						Layout: VBox{MarginsZero: true, Spacing: 2},
						Children: []Widget{
							Label{Text: "SmartPrint Agent", Font: Font{Family: "Segoe UI", PointSize: 14, Bold: true}},
							Label{Text: "Counter Print Automation • Silent Hardware Routing & Secure Shredding", Font: Font{Family: "Segoe UI", PointSize: 9}},
						},
					},
					HSpacer{},
					Composite{
						Layout: VBox{MarginsZero: true, Spacing: 2},
						Children: []Widget{
							Label{
								AssignTo: &headerConnLabel,
								Text:     "○ Connecting...",
								Font:     Font{Family: "Segoe UI", PointSize: 9, Bold: true},
							},
							Label{
								AssignTo: &headerStatsLabel,
								Text:     "Total: 0  |  Pending: 0  |  Active: 0  |  Done: 0",
								Font:     Font{Family: "Segoe UI", PointSize: 9},
							},
						},
					},
				},
			},


			// Main Tab View
			TabWidget{
				StretchFactor: 1,
				Font:          Font{Family: "Segoe UI", PointSize: 9},
				Pages: []TabPage{
					// TAB 1: LIVE QUEUE
					{
						Title:  "Live Print Queue",
						Layout: VBox{Margins: Margins{Left: 16, Top: 12, Right: 16, Bottom: 12}, Spacing: 8},
						Children: []Widget{
							// In-Window Alert Banner (shown when a job requires attention)
							Composite{
								AssignTo:   &alertBanner,
								Visible:    false,
								MinSize:    Size{Height: 40},
								MaxSize:    Size{Height: 40},
								Background: SolidColorBrush{Color: walk.RGB(254, 243, 199)},
								Layout:     HBox{Margins: Margins{Left: 12, Top: 6, Right: 12, Bottom: 6}, Spacing: 8},
								Children: []Widget{
									Label{AssignTo: &alertIconLabel, Text: "⚠️", Font: Font{Family: "Segoe UI", PointSize: 11}},
									Label{
										AssignTo: &alertTextLabel,
										Text:     "Action required",
										Font:     Font{Family: "Segoe UI", PointSize: 9, Bold: true},
									},
									HSpacer{},
									PushButton{
										AssignTo: &alertActionBtn,
										Text:     "Approve & Print",
										OnClicked: func() {
											if pendingJob, ok := model.FirstPendingJob(); ok {
												cfg := app.GetConfig()
												if isManualPayment(pendingJob.PaymentMethod) && !cfg.AutoAcceptUPI {
													promptUPIPaymentConfirmation(mw, app, pendingJob)
												} else {
													_ = app.ConfirmJob(pendingJob.ID)
												}
												updateDashboard()
											}
										},
									},
									PushButton{
										AssignTo: &alertDismissBtn,
										Text:     "✕ Dismiss Banner",
										OnClicked: func() {
											alertBanner.SetVisible(false)
										},
									},
								},
							},

							// Background Progress Indicator (active during download / print / preview)
							Composite{
								AssignTo: &progressComposite,
								Visible:  false,
								Layout:   VBox{MarginsZero: true, Spacing: 2},
								Children: []Widget{
									Label{AssignTo: &progressLabel, Text: "", Font: Font{Family: "Segoe UI", PointSize: 9}},
									ProgressBar{AssignTo: &progressBar, MarqueeMode: true, MinValue: 0, MaxValue: 100},
								},
							},

							// Section Bar
							Composite{
								Layout: HBox{MarginsZero: true},
								Children: []Widget{
									Label{Text: "Print Activity", Font: Font{Family: "Segoe UI", PointSize: 10, Bold: true}},
									HSpacer{},
									Label{Text: "Double-click any item to preview or confirm", Font: Font{Family: "Segoe UI", PointSize: 9}},
								},
							},

							// Main Data Table
							TableView{
								AssignTo:            &queueTable,
								AlternatingRowBG:    true,
								ColumnsOrderable:    true,
								ColumnsSizable:      true,
								LastColumnStretched: true,
								StretchFactor:       1,
								Font:                Font{Family: "Segoe UI", PointSize: 9},
								Columns: []TableViewColumn{
									{Title: "Document Name", Width: 230, Alignment: AlignNear},
									{Title: "Pages × Copies", Width: 105, Alignment: AlignCenter},
									{Title: "Mode", Width: 80, Alignment: AlignCenter},
									{Title: "Amount", Width: 90, Alignment: AlignFar},
									{Title: "Assigned Printer", Width: 160, Alignment: AlignNear},
									{Title: "Status", Width: 210, Alignment: AlignNear},
								},
								Model:     model,
								StyleCell: model.StyleCell,
								OnSelectedIndexesChanged: func() {
									updateSelectionState(queueTable, model, previewBtn, confirmBtn, retryBtn, selectionHintLabel)
								},
								OnItemActivated: func() {
									idx := queueTable.CurrentIndex()
									if job, ok := model.JobAt(idx); ok {
										if job.Status == queue.StatusQueued && job.PendingConfirmation {
											cfg := app.GetConfig()
											if isManualPayment(job.PaymentMethod) && !cfg.AutoAcceptUPI {
												promptUPIPaymentConfirmation(mw, app, job)
											} else {
												handleConfirm(app, queueTable, model)
											}
										} else if job.Status == queue.StatusFailed || job.Status == queue.StatusPrinterOffline {
											handleRetry(app, queueTable, model)
										} else {
											handlePreview(app, queueTable, model, mw, progressComposite, progressBar, progressLabel, previewBtn)
										}
										updateDashboard()
									}
								},
							},

							// Action Toolbar
							Composite{
								Layout: HBox{Margins: Margins{Top: 4}},
								Children: []Widget{
									Label{AssignTo: &selectionHintLabel, Text: "Select a print job from the table to take action."},
									HSpacer{},
									PushButton{
										AssignTo: &previewBtn,
										Enabled:  false,
										Text:     "👁️ Preview document",
										OnClicked: func() {
											handlePreview(app, queueTable, model, mw, progressComposite, progressBar, progressLabel, previewBtn)
										},
									},
									PushButton{
										AssignTo: &confirmBtn,
										Enabled:  false,
										Text:     "🖨️ Print now",
										OnClicked: func() {
											handleConfirm(app, queueTable, model)
											updateDashboard()
										},
									},
									PushButton{
										AssignTo: &retryBtn,
										Enabled:  false,
										Text:     "🔄 Retry failed",
										OnClicked: func() {
											handleRetry(app, queueTable, model)
											updateDashboard()
										},
									},
									PushButton{
										AssignTo: &clearBtn,
										Text:     "🧹 Clear completed",
										OnClicked: func() {
											model.ClearCompleted()
											updateDashboard()
										},
									},
								},
							},
						},
					},

					// TAB 2: SETTINGS
					{
						Title:  "Settings & Routing",
						Layout: VBox{MarginsZero: true, SpacingZero: true},
						Children: []Widget{
							ScrollView{
								HorizontalFixed: true,
								Layout:          VBox{Margins: Margins{Left: 20, Top: 16, Right: 20, Bottom: 20}, Spacing: 12},
								Children: []Widget{
									Label{Text: "Workstation Configuration", Font: Font{Family: "Segoe UI", PointSize: 11, Bold: true}},
									Label{Text: "Configure your shop identification, realtime credentials, and hardware printer routing below."},

									GroupBox{
										Title:  "1. Shop Identification",
										Layout: Grid{Columns: 2, Margins: Margins{Left: 14, Top: 14, Right: 14, Bottom: 12}, Spacing: 8},
										Children: []Widget{
											Label{Text: "Shop ID", MinSize: Size{Width: 150}},
											LineEdit{AssignTo: &shopIDEdit, ToolTipText: "Unique shop identifier registered on the platform"},
											Label{Text: "Access Token", MinSize: Size{Width: 150}},
											LineEdit{AssignTo: &authTokenEdit, PasswordMode: true, ToolTipText: "Bearer token used to authenticate Pusher channels"},
										},
									},

									GroupBox{
										Title:  "2. Realtime WebSocket Connection (Pusher)",
										Layout: Grid{Columns: 2, Margins: Margins{Left: 14, Top: 14, Right: 14, Bottom: 12}, Spacing: 8},
										Children: []Widget{
											Label{Text: "Pusher App Key", MinSize: Size{Width: 150}},
											LineEdit{AssignTo: &pusherKeyEdit},
											Label{Text: "Pusher Cluster", MinSize: Size{Width: 150}},
											LineEdit{AssignTo: &pusherClusterEdit},
											Label{Text: "Auth Endpoint URL", MinSize: Size{Width: 150}},
											LineEdit{AssignTo: &pusherAuthURLEdit, ToolTipText: "e.g. https://your-domain.com/api/pusher/auth"},
										},
									},

									GroupBox{
										Title:  "3. Hardware Printer Routing",
										Layout: Grid{Columns: 2, Margins: Margins{Left: 14, Top: 14, Right: 14, Bottom: 12}, Spacing: 8},
										Children: []Widget{
											Label{Text: "Black & White Printer", MinSize: Size{Width: 150}},
											ComboBox{AssignTo: &bwPrinterBox, Editable: false},
											Label{Text: "Color Printer", MinSize: Size{Width: 150}},
											ComboBox{AssignTo: &colorPrinterBox, Editable: false},
										},
									},

									Composite{
										Layout: HBox{Margins: Margins{Left: 4}},
										Children: []Widget{
											PushButton{
												Text: "🔄 Refresh installed printers",
												OnClicked: func() {
													loadPrintersIntoBoxes(app, bwPrinterBox, colorPrinterBox, printerStatusLabel)
												},
											},
											Label{AssignTo: &printerStatusLabel, Text: ""},
											HSpacer{},
										},
									},

									GroupBox{
										Title:  "4. Automation & Startup Options",
										Layout: VBox{Margins: Margins{Left: 14, Top: 14, Right: 14, Bottom: 12}, Spacing: 6},
										Children: []Widget{
											CheckBox{
												AssignTo: &autoPrintCheck,
												Text:     "Automatically print standard jobs upon receipt (Silent Auto-Print)",
											},
											Label{
												Text: "When unchecked, jobs pause in the Live Queue for manual confirmation before printing.",
											},
											VSpacer{Size: 4},
											CheckBox{
												AssignTo: &autoAcceptUPICheck,
												Text:     "Auto-accept Counter Pay & direct UPI Pay (print without confirmation popup)",
											},
											Label{
												Text: "When checked, UPI and counter payment jobs print directly without showing the confirmation popup.",
											},
											VSpacer{Size: 4},
											CheckBox{
												AssignTo: &autoStartCheck,
												Text:     "Launch automatically on Windows startup (Auto-Start in system tray)",
											},
											Label{
												Text: "Starts SmartPrint Agent minimized in the system tray whenever Windows boots up.",
											},
										},
									},

									Composite{
										Layout: HBox{Margins: Margins{Top: 6}},
										Children: []Widget{
											PushButton{
												Text: "💾 Save Configuration",
												OnClicked: func() {
													saveSettings(app, shopIDEdit, authTokenEdit,
														pusherKeyEdit, pusherClusterEdit, pusherAuthURLEdit,
														bwPrinterBox, colorPrinterBox, autoPrintCheck, autoAcceptUPICheck, autoStartCheck, saveStatusLabel, updateDashboard)
												},
											},
											Label{AssignTo: &saveStatusLabel, Text: ""},
											HSpacer{},
										},
									},

									VSpacer{Size: 8},
									Composite{
										Layout: HBox{MarginsZero: true},
										Children: []Widget{
											Label{
												Text: "SmartPrint Agent v1.1.0 • Publisher: BiswajitN99 • Website: https://biswajitn.in",
												Font: Font{Family: "Segoe UI", PointSize: 8},
											},
											HSpacer{},
										},
									},
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
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}

		mw.SetBounds(walk.Rectangle{
			X:      int(x),
			Y:      int(y),
			Width:  mw.Width(),
			Height: mw.Height(),
		})
	}

	// Minimize to tray instead of exiting, unless Quit was chosen from the tray menu.
	mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		if quitting {
			return
		}
		*canceled = true
		mw.Hide()
	})

	if appIcon, err := loadAppIcon(); err == nil && appIcon != nil {
		mw.SetIcon(appIcon)
		if logoImageView != nil {
			_ = logoImageView.SetImage(appIcon)
		}
	}

	if queueTable != nil {
		queueTable.SetGridlines(true)
	}

	if _, err := setupTray(mw, &quitting); err != nil {
		log.Printf("tray setup failed (continuing without tray icon): %v", err)
	}

	loadSettingsIntoForm(app, shopIDEdit, authTokenEdit, pusherKeyEdit, pusherClusterEdit,
		pusherAuthURLEdit, bwPrinterBox, colorPrinterBox, autoPrintCheck, autoAcceptUPICheck, autoStartCheck, printerStatusLabel)

	model.SetJobs(app.GetQueue())

	// Listen for live connection changes
	app.OnConnectionStateChange(func(state realtime.ConnectionState) {
		mw.Synchronize(func() {
			updateDashboard()
		})
	})

	// Listen for job status changes from print worker pool
	app.OnStatusChange(func(job queue.PrintJob) {
		mw.Synchronize(func() {
			model.Upsert(job)
			updateDashboard()
			updateSelectionState(queueTable, model, previewBtn, confirmBtn, retryBtn, selectionHintLabel)

			cfg := app.GetConfig()
			if job.Status == queue.StatusQueued && job.PendingConfirmation && isManualPayment(job.PaymentMethod) && !cfg.AutoAcceptUPI {
				promptUPIPaymentConfirmation(mw, app, job)
			}
		})
	})

	// Initial dashboard sync
	updateDashboard()

	mw.Run()
	return nil
}

func updateSelectionState(
	table *walk.TableView, model *jobTableModel,
	previewBtn, confirmBtn, retryBtn *walk.PushButton,
	selectionHintLabel *walk.Label,
) {
	idx := table.CurrentIndex()
	job, ok := model.JobAt(idx)
	if !ok {
		previewBtn.SetEnabled(false)
		confirmBtn.SetEnabled(false)
		retryBtn.SetEnabled(false)
		selectionHintLabel.SetText("Select a print job from the table to take action.")
		return
	}

	previewBtn.SetEnabled(true)

	if job.Status == queue.StatusQueued && job.PendingConfirmation {
		confirmBtn.SetEnabled(true)
		confirmBtn.SetText("🖨️ Print now")
	} else {
		confirmBtn.SetEnabled(false)
		confirmBtn.SetText("Print now")
	}

	if job.Status == queue.StatusFailed || job.Status == queue.StatusPrinterOffline {
		retryBtn.SetEnabled(true)
	} else {
		retryBtn.SetEnabled(false)
	}

	selectionHintLabel.SetText(fmt.Sprintf("Selected: %s [%s] — %s", job.Filename, job.ServiceCode, statusLabel(job)))
}

func loadSettingsIntoForm(
	app *App,
	shopIDEdit, authTokenEdit, pusherKeyEdit, pusherClusterEdit, pusherAuthURLEdit *walk.LineEdit,
	bwPrinterBox, colorPrinterBox *walk.ComboBox,
	autoPrintCheck, autoAcceptUPICheck, autoStartCheck *walk.CheckBox,
	printerStatusLabel *walk.Label,
) {
	cfg := app.GetConfig()
	shopIDEdit.SetText(cfg.ShopID)
	authTokenEdit.SetText(cfg.AuthToken)
	pusherKeyEdit.SetText(cfg.PusherAppKey)
	pusherClusterEdit.SetText(cfg.PusherCluster)
	pusherAuthURLEdit.SetText(cfg.PusherAuthURL)
	autoPrintCheck.SetChecked(cfg.SilentAutoPrint)
	autoAcceptUPICheck.SetChecked(cfg.AutoAcceptUPI)
	autoStartCheck.SetChecked(IsAutoStartEnabled())

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
	autoPrintCheck, autoAcceptUPICheck, autoStartCheck *walk.CheckBox,
	statusLabel *walk.Label,
	onSuccess func(),
) {
	cfg := config.Config{
		ShopID:            strings.TrimSpace(shopIDEdit.Text()),
		AuthToken:         strings.TrimSpace(authTokenEdit.Text()),
		PusherAppKey:      strings.TrimSpace(pusherKeyEdit.Text()),
		PusherCluster:     strings.TrimSpace(pusherClusterEdit.Text()),
		PusherAuthURL:     strings.TrimSpace(pusherAuthURLEdit.Text()),
		BlackWhitePrinter: comboText(bwPrinterBox),
		ColorPrinter:      comboText(colorPrinterBox),
		SilentAutoPrint:   autoPrintCheck.Checked(),
		AutoAcceptUPI:     autoAcceptUPICheck.Checked(),
	}

	if err := app.SaveConfig(cfg); err != nil {
		statusLabel.SetText(fmt.Sprintf("❌ Error: %v", err))
		return
	}

	// Persist Windows startup registry setting
	if err := SetAutoStart(autoStartCheck.Checked()); err != nil {
		log.Printf("failed to update autostart setting: %v", err)
	}

	statusLabel.SetText("✅ Configuration saved.")
	if onSuccess != nil {
		onSuccess()
	}
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

func handlePreview(
	app *App, table *walk.TableView, model *jobTableModel,
	mw *walk.MainWindow, progressComposite *walk.Composite,
	progressBar *walk.ProgressBar, progressLabel *walk.Label,
	previewBtn *walk.PushButton,
) {
	idx := table.CurrentIndex()
	job, ok := model.JobAt(idx)
	if !ok {
		return
	}

	previewBtn.SetEnabled(false)
	if progressComposite != nil {
		progressComposite.SetVisible(true)
	}
	if progressBar != nil {
		_ = progressBar.SetMarqueeMode(true)
	}
	if progressLabel != nil {
		progressLabel.SetText(fmt.Sprintf("Opening secure preview for '%s'...", job.Filename))
	}

	go func() {
		err := app.PreviewJob(job.ID)
		mw.Synchronize(func() {
			previewBtn.SetEnabled(true)
			if progressComposite != nil {
				progressComposite.SetVisible(false)
			}
			if err != nil {
				walk.MsgBox(mw, "Preview Error", fmt.Sprintf("Failed to preview document:\n%v", err), walk.MsgBoxIconError)
			}
		})
	}()
}

func promptUPIPaymentConfirmation(mw walk.Form, app *App, job queue.PrintJob) {
	colorStr := "Black & White"
	if job.IsColor {
		colorStr = "Color"
	}
	code := job.ServiceCode
	if code == "" {
		code = "N/A"
	}

	message := fmt.Sprintf(
		"UPI Payment Verification Required\n\n"+
			"• Document:     %s\n"+
			"• Order Code:   %s\n"+
			"• Color Mode:   %s\n"+
			"• Details:      %d page(s) × %d copie(s)\n"+
			"• Total Due:    INR %.2f\n\n"+
			"Please verify that this exact amount has arrived in the shop's UPI account / Soundbox.\n\n"+
			"Do you want to release and print this document now?",
		job.Filename, code, colorStr, job.Pages, job.Copies, job.TotalAmount,
	)

	if walk.MsgBox(mw, "Confirm UPI Payment — SmartPrint", message, walk.MsgBoxYesNo|walk.MsgBoxIconInformation) == walk.DlgCmdYes {
		if err := app.ConfirmJob(job.ID); err != nil {
			log.Printf("UPI print confirmation failed: %v", err)
			walk.MsgBox(mw, "Print Release Error", fmt.Sprintf("Failed to print job:\n%v", err), walk.MsgBoxIconError)
		}
	}
}
