# Dopo Goto

Dopo Goto is a terminal-native audiovisual experience.
15 albums. 30+ hours of music. Live chat.
All in your command line.

Welcome to A Brand New World!

![Dopo Goto Preview](dopogoto-preview.gif)

## Installation

### Ask AI

Paste this into Claude Code, Codex, Cursor, or any AI agent with terminal access:

> Install dopogoto terminal music player from github.com/dangerous-person/dopogoto

### macOS (recommended)

1. Open **Terminal** (press `Command + Space`, type `Terminal`, press Enter).
2. Paste this command and press Enter:

```sh
curl -fsSL https://raw.githubusercontent.com/dangerous-person/dopogoto/main/install.sh | sh
```

3. If prompted, type your Mac password to allow install to `/usr/local/bin`.
   (No characters appear while typing. This is normal.)
4. Start the app:

```sh
dopogoto
```

If Terminal says `command not found`, run:

```sh
/usr/local/bin/dopogoto
```

Then close and reopen Terminal.

### macOS (manual download)

1. On [Releases](https://github.com/dangerous-person/dopogoto/releases), download the macOS file:
   `dopogoto_*_darwin_universal.tar.gz`
   (`darwin` means macOS, `universal` means it works on both Intel and Apple Silicon Macs).
2. Open Downloads in Finder and double-click the `.tar.gz` file.
   This creates a folder like `dopogoto_0.1.6_darwin_universal` containing the `dopogoto` app.
3. In Terminal, run:

```sh
sudo mv ~/Downloads/dopogoto_*_darwin_universal/dopogoto /usr/local/bin/dopogoto
sudo chmod +x /usr/local/bin/dopogoto
xattr -d com.apple.quarantine /usr/local/bin/dopogoto 2>/dev/null || true
dopogoto
```

Keep `*` exactly as shown (do not replace it). It automatically matches the version folder name.

If you have multiple `dopogoto_*_darwin_universal` folders in Downloads, use the exact one from Finder, for example:

```sh
sudo mv ~/Downloads/dopogoto_0.1.6_darwin_universal/dopogoto /usr/local/bin/dopogoto
```

### Linux

```sh
curl -fsSL https://raw.githubusercontent.com/dangerous-person/dopogoto/main/install.sh | sh
```

### Windows

1. Download `dopogoto_*_windows_amd64.zip` from [Releases](https://github.com/dangerous-person/dopogoto/releases)
2. Extract, and run `dopogoto.exe`.

### Build from source

Requires Go 1.25+:

```sh
go install github.com/dangerous-person/dopogoto@latest
```

## Requirements

- A 256-color terminal (Ghostty, Terminal.app, iTerm2, Windows Terminal, etc.)
- Terminal size 120x40 or larger
- Audio output device

**Linux:** ALSA is required for audio playback.
- Debian/Ubuntu: `sudo apt install libasound2-dev`
- Fedora: `sudo dnf install alsa-lib-devel`
- Arch: `sudo pacman -S alsa-lib`

## Controls

| Key | Action |
|-----|--------|
| TAB | Cycle focus: Albums / Tracks / Chat |
| UP/DOWN | Navigate lists |
| ENTER | Play track or send chat message |
| SPACE | Pause / resume |
| N / P | Next / previous track |
| +/- | Volume up / down |
| S | Shuffle |
| R | Repeat |
| T | Change theme |
| LEFT/RIGHT | Seek -/+ 10s |
| Q | Quit |

## Chat

Type a message and press Enter.

- `/nick name` -- set your nickname (saved locally)
- `/reset` -- go anonymous

## Telemetry

App sends a single anonymous ping on launch (version, OS) to help us understand usage. No personal info. No IP tracking.

To opt out: `export DOPOGOTO_NO_TELEMETRY=1`

## Licensing

- Source code is licensed under MIT (see `LICENSE`).
- Media/assets are licensed separately (see [ASSETS_LICENSE.md](ASSETS_LICENSE.md)) and are not covered by MIT.

## Links
- https://www.youtube.com/@dopogoto
- https://dopogoto.bandcamp.com
- https://dopogoto.com
