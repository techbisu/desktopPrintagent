# SmartPrint Desktop Agent (Go + walk, native Windows UI)

Windows background agent for the QR-to-Print system. Listens for print jobs
over Pusher, downloads the file, prints it silently to the correct
hardware printer (B&W vs Color), and shreds the temp file immediately after.

The UI is built with [`lxn/walk`](https://github.com/lxn/walk), a native
Win32 GUI toolkit for Go — no embedded browser engine, no WebView2, no
OpenGL. This is a deliberate choice: it's the only approach that reliably
runs on **Windows 7, 8, 10, and 11 alike**, since WebView2 (used by
Electron/Wails-style apps) dropped Windows 7/8 support in January 2023,
and OpenGL-based toolkits can fail on older machines or VMs with minimal
display drivers. `walk` talks straight to GDI/User32, the same rendering
path Windows itself has used for decades, so if the OS can draw a window
at all, this app runs.

## Before you build

This was written and reviewed outside of a Go/Windows environment, so
three things need your attention before it will compile and run for real:

1. **`internal/printer/bin/SumatraPDF.exe`** — currently a 0-byte
   placeholder (see `internal/printer/bin/README.md`). Download the
   **portable** 64-bit SumatraPDF build and replace it, keeping the
   filename exactly `SumatraPDF.exe`.
2. **`build/windows/icon.ico`** — currently a 0-byte placeholder. Drop in
   a real multi-resolution `.ico` for the tray icon.
3. **`go.sum`** — not included, since this environment has no network
   access to the Go module proxy. Run `go mod tidy` once you're on a
   machine with internet access; it will fetch and pin:
   - `github.com/lxn/walk` and `github.com/lxn/win` (native UI)
   - `github.com/gorilla/websocket` (Pusher realtime client)
   - `github.com/google/uuid`
   - `golang.org/x/sys` (single-instance mutex)

There is no Node.js, npm, or frontend build step anymore — this is a
single Go binary, which is part of why it's lightweight.

## Project layout

```
main.go                          Entry point: single-instance check, starts App and UI
app.go                           Business logic: config, print pipeline, Pusher — no UI dependency
ui.go                            Native window layout (Live Queue + Settings tabs), built with walk
tablemodel.go                    walk TableModel backing the Live Queue TableView
tray.go                          System tray icon and Show/Quit menu (walk.NotifyIcon)
singleinstance.go                Named-mutex single-instance guard
internal/config/                 Settings persistence (%AppData%/SmartPrint/config.json)
internal/printer/                Embedded SumatraPDF + silent print execution
internal/hardware/                Windows printer enumeration via PowerShell
internal/queue/                  Buffered worker pool: download -> process -> print -> shred
internal/realtime/                Hand-rolled Pusher Channels websocket client
internal/docprocessor/           DocumentProcessor interface (Phase 2 hook lives here)
```

## Backend endpoint this agent expects

`internal/realtime` calls your Next.js backend's Pusher auth endpoint
(the `pusherAuthUrl` set in the Settings tab) exactly the way `pusher-js`
does for private channels:

```
POST {pusherAuthUrl}
Authorization: Bearer {authToken}
Content-Type: application/json

{ "socket_id": "...", "channel_name": "private-shop-<shopId>" }
```

Expected response:

```json
{ "auth": "your_app_key:generated_hmac_signature" }
```

Your Next.js API route should look up the shop by `authToken`, verify it,
then sign the auth string server-side using your Pusher app secret (never
ship the secret to the desktop agent). It publishes `new-print-job` events
to `private-shop-<shopId>` with a payload matching
`internal/queue.PrintJob`'s JSON tags (`id`, `serviceCode`, `filename`,
`fileUrl`, `fileType`, `pages`, `copies`, `isColor`, `isDuplex`,
`totalAmount`).

## Build commands

```bash
# One-time Go deps (needs internet access)
go mod tidy

# Build the Windows executable directly
go build -ldflags="-H=windowsgui -s -w" -o build/bin/SmartPrintAgent.exe .

# Run it
./build/bin/SmartPrintAgent.exe
```

`-H=windowsgui` suppresses the console window; `-s -w` strips debug info
to keep the binary small.

## Notes on design decisions

- **Native UI, not a browser**: chosen specifically so the agent runs
  identically on Windows 7 through 11, including on lower-spec or virtual
  machines, without depending on a runtime (WebView2) or GPU capability
  (OpenGL) that isn't guaranteed to be present.
- **Silent printing**: every subprocess call (SumatraPDF, PowerShell) sets
  `HideWindow: true` + `CREATE_NO_WINDOW` so nothing ever flashes on
  screen or steals focus.
- **Privacy shredding**: `internal/queue/worker.go` uses `defer
  os.Remove(...)` immediately after download, before print even runs, so
  the temp file is removed on every code path — success, print failure, or
  processing failure alike.
- **Reconnect backoff**: `internal/realtime` doubles its retry delay up to
  a 60s cap after each dropped connection.
- **Single instance + tray**: a named Windows mutex (`singleinstance.go`)
  prevents a second process from starting; the window's `Closing` handler
  hides it instead of exiting; `walk.NotifyIcon` provides the tray icon
  and Show/Quit menu.
- **UI thread safety**: `queue.Manager` emits status changes from
  background worker goroutines. `ui.go` marshals every such update onto
  the UI thread via `mw.Synchronize(...)` before touching any widget —
  required because `walk`, like most native GUI toolkits, is not
  thread-safe.
- **Phase 2 hook**: `internal/docprocessor.DocumentProcessor` is the seam
  for a future legal-document processor (e.g. overlaying filled-in fields
  onto a stamp-paper PDF template) — nothing else in the pipeline needs to
  change.
