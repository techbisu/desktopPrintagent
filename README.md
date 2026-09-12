# SmartPrint Desktop Agent (Wails v2 + Go)

Windows background agent for the QR-to-Print system. Listens for print jobs
over Pusher, downloads the file, prints it silently to the correct
hardware printer (B&W vs Color), and shreds the temp file immediately after.

## Before you build

This was written and reviewed outside of a Go/Windows environment, so three
things need your attention before it will compile and run for real:

1. **`internal/printer/bin/SumatraPDF.exe`** — currently a 0-byte
   placeholder (see `internal/printer/bin/README.md`). Download the
   **portable** 64-bit SumatraPDF build and replace it, keeping the
   filename exactly `SumatraPDF.exe`.
2. **`build/windows/icon.ico`** — currently a 0-byte placeholder. Drop in a
   real multi-resolution `.ico` for the tray icon / exe icon.
3. **`go.sum`** — not included, since this environment has no network
   access to the Go module proxy. Run `go mod tidy` once you're on a
   machine with internet access; it will fetch and pin:
   - `github.com/wailsapp/wails/v2`
   - `github.com/getlantern/systray`
   - `github.com/gorilla/websocket`
   - `github.com/google/uuid`

## Project layout

```
main.go                          Wails entrypoint, window/tray/single-instance config
app.go                           App struct bound to the frontend, Pusher wiring
internal/config/                 Settings persistence (%AppData%/SmartPrint/config.json)
internal/printer/                Embedded SumatraPDF + silent print execution
internal/hardware/               Windows printer enumeration via PowerShell
internal/queue/                  Buffered worker pool: download -> process -> print -> shred
internal/realtime/               Hand-rolled Pusher Channels websocket client
internal/docprocessor/           DocumentProcessor interface (Phase 2 DOCX hook lives here)
frontend/                        React + Tailwind UI (Settings tab, Live Queue tab)
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
# One-time frontend deps
cd frontend && npm install && cd ..

# One-time Go deps (needs internet access)
go mod tidy

# Dev mode with hot reload
wails dev

# Production build (silent GUI subsystem, stripped binary)
wails build -ldflags="-H=windowsgui -s -w"
```

The compiled binary lands in `build/bin/SmartPrintAgent.exe`.

## Notes on design decisions

- **Silent printing**: every subprocess call (SumatraPDF, PowerShell) sets
  `HideWindow: true` + `CREATE_NO_WINDOW` so nothing ever flashes on
  screen or steals focus.
- **Privacy shredding**: `internal/queue/worker.go` uses `defer
  os.Remove(...)` immediately after download, before print even runs, so
  the temp file is removed on every code path — success, print failure, or
  processing failure alike.
- **Reconnect backoff**: `internal/realtime` doubles its retry delay up to
  a 60s cap after each dropped connection.
- **Single instance + tray**: `SingleInstanceLock` (Wails) prevents a
  second process from starting; `OnBeforeClose` hides the window instead of
  quitting; `getlantern/systray` provides the tray icon and Show/Quit menu.
- **Phase 2 hook**: `internal/docprocessor.DocumentProcessor` is the seam
  for a future `LibreOfficeProcessor` that converts legal DOCX templates to
  PDF before printing — nothing else in the pipeline needs to change.
