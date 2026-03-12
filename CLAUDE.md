# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```sh
go build -o dopogoto .
./dopogoto
```

The binary requires a real TTY — it cannot run in non-interactive environments.

**With version injection (mirrors release builds):**
```sh
go build -ldflags="-X main.version=dev" -o dopogoto .
```

**Tests:**
```sh
go test ./...
go test ./internal/chat/...          # specific package
go test -run TestSanitize ./internal/chat/
```

**Linux requires ALSA headers to build:**
```sh
sudo apt-get install libasound2-dev   # Debian/Ubuntu
```

## Architecture

The app is a [Bubbletea](https://github.com/charmbracelet/bubbletea) TUI following the Elm architecture (Model/Update/View).

### Data flow

```
main.go
  └─ ui.App (root bubbletea model)
       ├─ player.Player          — downloads MP3 from CDN, plays via beep/oto, sends msgs to App
       ├─ chat.Client            — Supabase REST + WebSocket realtime, sends msgs to App
       ├─ panels.Video           — animates ascii-art video clips at ~30fps
       ├─ panels.AlbumList       — left column top; navigates data.Albums
       ├─ panels.TrackList       — left column bottom
       ├─ panels.Chat            — right column; renders messages, handles text input
       └─ panels.Controls        — bottom bar; shows track title, seek bar, volume
```

Async components (`player.Player`, `chat.Client`) communicate back to the bubbletea loop via a `sendMsg func(interface{})` injected by `App.SetProgram()`. This avoids direct bubbletea imports in inner packages.

### Key packages

- **`internal/data`** — static album/track catalog; CDN URLs are constructed at init time. `ShuffledAlbums()` randomizes order on each launch.
- **`internal/player`** — downloads MP3 fully into memory (≤50 MB), decodes with `go-mp3`, plays via `gopxl/beep`. Audio chain: `StreamSeekCloser → beep.Ctrl → effects.Volume → speaker`. Sends `TrackStartedMsg`, `ProgressMsg`, `TrackEndMsg`, `ErrorMsg`, `BufferingMsg` to App.
- **`internal/chat`** — Supabase backend. Fetches last 100 messages on start via REST, then subscribes to realtime INSERTs via Supabase's Phoenix WebSocket protocol. Reconnects automatically.
- **`internal/video`** — decodes brotli-compressed JSON video files embedded via `//go:embed`. Supports v1 (int arrays) and v2 (base-93 encoded) frame formats; keyframe + delta compression. Each `assets/NNN.json.br` is one video clip.
- **`internal/theme`** — HSL↔RGB↔256-color conversion utilities; used for dynamic gradient generation in panels.
- **`internal/ui/panels`** — individual TUI panel renderers. Each panel is a struct with `Width`/`Height`/`Focused` fields and a `View() string` method. ANSI escape codes are used directly (not lipgloss) for rendering. `render.go` has shared border/title helpers. `theme.go` manages the current theme and `CycleTheme()`.

### Key bindings (implemented in `internal/ui/app.go`)

Navigation keys are handled in `App.Update()`. Non-chat focus (`focusAlbums`, `focusTracks`) and chat focus (`focusChat`) have separate key handlers. Notable shortcuts:
- `g` / `G` — jump to top/bottom of Albums or Songs list (vim-style)
- `ctrl+u` / `ctrl+d` — scroll Chat history to top/bottom (vim/less-style)

### Layout

`App.View()` produces: `[video | albumList] / [chat | trackList]` side-by-side (left column: `video.FrameWidth()` wide), then controls bar, then help bar. Minimum terminal size is 120×40; if smaller, an animated "too small" video plays.

### Releases

Tags (`v*`) trigger the GitHub Actions release workflow: Linux built on Ubuntu with CGO/ALSA, macOS/Windows via `goreleaser`. The `version` variable in `main.go` is set via `-ldflags="-X main.version=..."`.

## Contributing

Fork `dangerous-person/dopogoto`, work in feature branches, open PRs against `main`.

CI runs on every PR and checks (in order):

```sh
gofmt -l .          # must produce no output — run `gofmt -w .` before committing
go vet ./...
go test ./...
go test -race ./...
go build .
```

Always run these locally before pushing. The race test requires `CGO_ENABLED=1`.

## Meta

- Keep this file up to date whenever the codebase changes.
- **Bump the version** in `main.go` (`var version = "x.y.z"`) with every commit. Current: `0.1.7`. Upstream latest tag: `v0.1.6`.
- **Always build** after every change: `go build -o dopogoto .`
