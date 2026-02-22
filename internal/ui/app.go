package ui

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/dangerous-person/dopogoto/assets"
	"github.com/dangerous-person/dopogoto/internal/chat"
	"github.com/dangerous-person/dopogoto/internal/data"
	"github.com/dangerous-person/dopogoto/internal/player"
	"github.com/dangerous-person/dopogoto/internal/ui/panels"
	"github.com/dangerous-person/dopogoto/internal/video"

	tea "github.com/charmbracelet/bubbletea"
)

// updateAvailableMsg is sent when a newer version is found on GitHub.
type updateAvailableMsg struct {
	Version string
}

// tickMsg drives animation
type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(33*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// focus tracks which panel has keyboard focus
type focus int

const (
	focusAlbums focus = iota
	focusTracks
	focusChat
)

// App is the root bubbletea model.
type App struct {
	video      panels.Video
	albumList  panels.AlbumList
	trackList  panels.TrackList
	chat       panels.Chat
	controls   panels.Controls
	player     *player.Player
	chatClient *chat.Client
	nickname   string
	focus      focus
	width      int
	height     int
	ready      bool
	animTick   int

	// Track navigation state
	currentAlbumIdx int
	currentTrackIdx int

	version string

	// Telemetry
	startTime    time.Time
	tracksPlayed int

	// Too-small screen video
	tsDec      *video.Decoder
	tsRen      *video.Renderer
	tsFrame    int
	tsAccum    float64
	tsFrameDur float64
}

func NewApp(version string) *App {
	vid, err := panels.NewVideo(assets.Video001BR, assets.Video002BR, assets.Video003BR, assets.Video004BR, assets.Video005BR, assets.Video006BR, assets.Video007BR, assets.Video008BR, assets.Video009BR, assets.Video010BR, assets.Video011BR, assets.Video012BR, assets.Video013BR, assets.Video014BR, assets.Video015BR, assets.Video016BR, assets.Video017BR, assets.Video018BR, assets.Video019BR, assets.Video020BR, assets.Video021BR, assets.Video022BR, assets.Video023BR, assets.Video024BR, assets.Video025BR, assets.Video026BR, assets.Video027BR, assets.Video028BR)
	if err != nil {
		log.Printf("video init: %v", err)
	}

	albums := data.ShuffledAlbums()
	al := panels.NewAlbumList(albums)
	al.Focused = true

	tl := panels.NewTrackList()
	cfg := chat.LoadConfig()

	app := &App{
		video:           vid,
		albumList:       al,
		trackList:       tl,
		chat:            panels.NewChat(),
		controls:        panels.NewControls(),
		player:          player.New(),
		chatClient:      chat.NewClient(),
		nickname:        cfg.Nickname,
		focus:           focusAlbums,
		version:         version,
		currentAlbumIdx: -1,
		currentTrackIdx: -1,
		startTime:       time.Now(),
	}

	// Too-small screen video
	if tsDec, err := video.NewDecoder(assets.TooSmallBR); err == nil {
		app.tsDec = tsDec
		app.tsRen = video.NewRenderer(tsDec.Data.Palette)
		fps := tsDec.FPS()
		if fps <= 0 {
			fps = 30
		}
		app.tsFrameDur = 1.0 / float64(fps)
		tsDec.ApplyFrame(0)
	}

	// Show tracks for the initially selected album
	if sel := al.SelectedAlbum(); sel != nil {
		app.trackList.SetAlbum(sel)
		app.trackList.Color = al.SelectedColor()
	}

	return app
}

// SetProgram gives the app a reference to the bubbletea program for async message routing.
func (a *App) SetProgram(p *tea.Program) {
	send := func(msg interface{}) {
		p.Send(msg)
	}
	a.player.SetSendFunc(send)
	a.chatClient.SetSendFunc(send)
}

func (a *App) Init() tea.Cmd {
	a.chatClient.Start()
	cmds := []tea.Cmd{tickCmd(), a.checkForUpdate()}
	if os.Getenv("DOPOGOTO_NO_TELEMETRY") == "" {
		go a.sendTelemetry()
	}
	return tea.Batch(cmds...)
}

func (a *App) sendTelemetry() {
	a.sendEvent("app_launched", fmt.Sprintf(`"version":"%s","os":"%s","arch":"%s"`,
		a.version, runtime.GOOS, runtime.GOARCH))
}

func (a *App) sendSessionTelemetry() {
	dur := int(time.Since(a.startTime).Seconds())
	theme := panels.CurrentTheme().Name
	a.sendEvent("app_session", fmt.Sprintf(`"duration_s":%d,"tracks_played":%d,"theme":"%s","terminal_w":%d,"terminal_h":%d`,
		dur, a.tracksPlayed, theme, a.width, a.height))
}

func (a *App) sendEvent(event, props string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	body := fmt.Sprintf(`{"api_key":"phc_1S0EQGoWYuoCNLBOsBimJbrpWUbveQfafLCmLAxWBhw","event":"%s","distinct_id":"dopogoto-terminal","properties":{%s}}`,
		event, props)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://us.i.posthog.com/capture/", strings.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(req)
}

func (a *App) checkForUpdate() tea.Cmd {
	if a.version == "dev" || a.version == "" {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, "GET",
			"https://api.github.com/repos/dangerous-person/dopogoto/releases/latest", nil)
		if err != nil {
			return nil
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != 200 {
			return nil
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		// Quick parse for "tag_name" without importing encoding/json
		const key = `"tag_name":"`
		idx := strings.Index(string(body), key)
		if idx < 0 {
			return nil
		}
		start := idx + len(key)
		end := strings.Index(string(body[start:]), `"`)
		if end < 0 {
			return nil
		}
		latest := string(body[start : start+end])
		if latest != a.version && latest > a.version {
			return updateAvailableMsg{Version: latest}
		}
		return nil
	}
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case updateAvailableMsg:
		a.chat.AddLocalMessage("[update]",
			msg.Version+" available — curl -fsSL .../install.sh | sh")
		return a, nil

	case tickMsg:
		a.video.Tick(33)
		a.tickTooSmallVideo(33)
		a.animTick++
		if a.animTick%6 == 0 && a.controls.State == panels.StatePlaying {
			a.trackList.AnimTick++
		}
		// Update controls with player state
		a.syncControlsFromPlayer()
		return a, tickCmd()

	case player.TrackStartedMsg:
		a.tracksPlayed++
		a.controls.State = panels.StatePlaying
		a.controls.TrackTitle = msg.TrackTitle
		a.controls.Duration = msg.Duration
		a.controls.Position = 0
		return a, nil

	case player.ProgressMsg:
		a.controls.Position = msg.Position
		if msg.Length > 0 {
			a.controls.Duration = msg.Length
		}
		return a, nil

	case player.TrackEndMsg:
		return a, a.playNext()

	case player.ErrorMsg:
		a.controls.State = panels.StateStopped
		a.controls.TrackTitle = "Couldn't load — skipping to next"
		// Auto-skip to next track after error
		return a, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return player.TrackEndMsg{}
		})

	case player.BufferingMsg:
		a.controls.State = panels.StateBuffering
		a.controls.TrackTitle = msg.TrackTitle
		return a, nil

	case chat.NewMessagesMsg:
		a.chat.SetMessages(msg.Messages)
		a.chat.SetOffline(false)
		return a, nil

	case chat.ChatOfflineMsg:
		a.chat.SetOffline(true)
		return a, nil

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height

		if a.width < 120 || a.height < 40 {
			a.ready = false
			return a, nil
		}

		// Controls at bottom (full width, fixed height)
		ctrlH := panels.ControlsHeight()
		availH := a.height - ctrlH - 1 // -1 for help bar
		if availH < 5 {
			availH = 5
		}
		a.controls.Width = a.width

		// Column widths
		videoW := a.video.FrameWidth()
		if videoW > a.width {
			videoW = a.width
		}
		rightW := a.width - videoW
		if rightW < 0 {
			rightW = 0
		}

		// Left column
		videoH := a.video.VideoHeight() + 2
		if videoH > availH {
			videoH = availH
		}
		chatH := availH - videoH
		if chatH < 0 {
			chatH = 0
		}

		a.video.Width = videoW
		a.video.Height = videoH
		a.chat.Width = videoW
		a.chat.Height = chatH

		// Right column
		const albumH = 17
		songsH := availH - albumH
		if songsH < 3 {
			songsH = 3
		}

		a.albumList.Width = rightW
		a.albumList.Height = albumH
		a.trackList.Width = rightW
		a.trackList.Height = songsH

		a.ready = true
		return a, nil

	case tea.KeyMsg:
		// Chat input mode
		if a.focus == focusChat {
			return a.handleChatKey(msg)
		}

		if isQuit(msg) {
			a.player.Close()
			a.chatClient.Stop()
			if os.Getenv("DOPOGOTO_NO_TELEMETRY") == "" {
				a.sendSessionTelemetry()
			}
			return a, tea.Quit
		}
		switch msg.String() {
		case "tab":
			a.switchFocus()
		case "up", "k":
			a.navigateUp()
		case "down", "j":
			a.navigateDown()
		case "enter":
			return a, a.handleEnter()
		case " ":
			a.togglePause()
		case "n":
			return a, a.playNext()
		case "p":
			return a, a.playPrev()
		case "+", "=":
			a.player.VolumeUp()
			a.controls.Volume = a.player.VolumeLevel()
		case "-":
			a.player.VolumeDown()
			a.controls.Volume = a.player.VolumeLevel()
		case "s":
			a.controls.Shuffle = !a.controls.Shuffle
			if a.controls.Shuffle {
				a.controls.Repeat = false
			}
		case "r":
			a.controls.Repeat = !a.controls.Repeat
			if a.controls.Repeat {
				a.controls.Shuffle = false
			}
		case "t":
			panels.CycleTheme()
		case ">":
			a.video.NextClip()
		case "left":
			// Seek back 10s
			pos := a.player.Position() - 10*time.Second
			if pos < 0 {
				pos = 0
			}
			a.player.Seek(pos)
		case "right":
			// Seek forward 10s
			pos := a.player.Position() + 10*time.Second
			a.player.Seek(pos)
		}
	}

	return a, nil
}

func (a *App) syncControlsFromPlayer() {
	if a.player.IsPlaying() && a.controls.State != panels.StatePlaying {
		a.controls.State = panels.StatePlaying
	}
	a.controls.Volume = a.player.VolumeLevel()
}

func (a *App) handleEnter() tea.Cmd {
	switch a.focus {
	case focusAlbums:
		// Switch to tracks panel for this album
		a.switchFocus()
		return nil
	case focusTracks:
		// Play selected track
		return a.playSelectedTrack()
	}
	return nil
}

func (a *App) playSelectedTrack() tea.Cmd {
	track := a.trackList.SelectedTrack()
	if track == nil {
		return nil
	}

	a.currentAlbumIdx = a.albumList.Cursor
	a.currentTrackIdx = a.trackList.Cursor
	a.trackList.PlayingTrack = a.currentTrackIdx

	a.controls.State = panels.StateBuffering
	a.controls.TrackTitle = track.Title
	a.controls.AlbumColor = a.trackList.Color
	a.controls.Position = 0
	a.controls.Duration = 0

	url := track.URL
	title := track.Title

	return func() tea.Msg {
		a.player.PlayURL(url, title)
		return player.BufferingMsg{TrackTitle: title}
	}
}

func (a *App) playNext() tea.Cmd {
	if a.currentAlbumIdx < 0 {
		return nil
	}

	// Repeat: replay same track
	if a.controls.Repeat {
		track := &a.albumList.Albums[a.currentAlbumIdx].Tracks[a.currentTrackIdx]
		a.controls.State = panels.StateBuffering
		a.controls.TrackTitle = track.Title
		a.controls.AlbumColor = a.trackList.Color
		a.controls.Position = 0

		url := track.URL
		title := track.Title
		return func() tea.Msg {
			a.player.PlayURL(url, title)
			return player.BufferingMsg{TrackTitle: title}
		}
	}

	var albumIdx, trackIdx int

	if a.controls.Shuffle {
		// Pick random album and track
		albumIdx = rand.Intn(len(a.albumList.Albums))
		trackIdx = rand.Intn(len(a.albumList.Albums[albumIdx].Tracks))
	} else {
		album := &a.albumList.Albums[a.currentAlbumIdx]
		trackIdx = a.currentTrackIdx + 1
		albumIdx = a.currentAlbumIdx

		if trackIdx >= len(album.Tracks) {
			albumIdx++
			if albumIdx >= len(a.albumList.Albums) {
				a.controls.State = panels.StateStopped
				a.controls.TrackTitle = ""
				return nil
			}
			trackIdx = 0
		}
	}

	a.currentAlbumIdx = albumIdx
	a.albumList.Cursor = albumIdx
	a.syncTracks()
	a.currentTrackIdx = trackIdx
	a.trackList.Cursor = trackIdx
	a.trackList.PlayingTrack = trackIdx

	track := &a.albumList.Albums[albumIdx].Tracks[trackIdx]
	a.controls.State = panels.StateBuffering
	a.controls.TrackTitle = track.Title
	a.controls.AlbumColor = a.trackList.Color
	a.controls.Position = 0

	url := track.URL
	title := track.Title

	return func() tea.Msg {
		a.player.PlayURL(url, title)
		return player.BufferingMsg{TrackTitle: title}
	}
}

func (a *App) playPrev() tea.Cmd {
	if a.currentAlbumIdx < 0 {
		return nil
	}

	// If more than 3s into the track, restart it
	if a.player.Position() > 3*time.Second {
		a.player.Seek(0)
		return nil
	}

	prevTrack := a.currentTrackIdx - 1
	if prevTrack < 0 {
		// Go to previous album
		prevAlbum := a.currentAlbumIdx - 1
		if prevAlbum < 0 {
			prevAlbum = len(a.albumList.Albums) - 1
		}
		a.currentAlbumIdx = prevAlbum
		a.albumList.Cursor = prevAlbum
		a.syncTracks()
		prevTrack = len(a.albumList.Albums[prevAlbum].Tracks) - 1
	}

	a.currentTrackIdx = prevTrack
	a.trackList.Cursor = prevTrack
	a.trackList.PlayingTrack = prevTrack

	track := &a.albumList.Albums[a.currentAlbumIdx].Tracks[prevTrack]
	a.controls.State = panels.StateBuffering
	a.controls.TrackTitle = track.Title
	a.controls.AlbumColor = a.trackList.Color
	a.controls.Position = 0

	url := track.URL
	title := track.Title

	return func() tea.Msg {
		a.player.PlayURL(url, title)
		return player.BufferingMsg{TrackTitle: title}
	}
}

func (a *App) togglePause() {
	if a.controls.State == panels.StateStopped || a.controls.State == panels.StateBuffering {
		return
	}
	playing := a.player.TogglePause()
	if playing {
		a.controls.State = panels.StatePlaying
	} else {
		a.controls.State = panels.StatePaused
	}
}

func (a *App) handleChatKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		a.player.Close()
		a.chatClient.Stop()
		if os.Getenv("DOPOGOTO_NO_TELEMETRY") == "" {
			a.sendSessionTelemetry()
		}
		return a, tea.Quit
	case "esc", "tab":
		a.switchFocus()
	case "enter":
		text := a.chat.ClearInput()
		if text != "" {
			return a, a.sendChatMsg(text)
		}
	case "backspace":
		a.chat.Backspace()
	case "up":
		a.chat.ScrollUp()
	case "down":
		a.chat.ScrollDown()
	case " ":
		// Space in chat types a space, but also toggle pause
		a.chat.InsertRune(' ')
	default:
		if msg.Type == tea.KeyRunes {
			for _, r := range msg.Runes {
				a.chat.InsertRune(r)
			}
		}
	}
	return a, nil
}

func (a *App) sendChatMsg(text string) tea.Cmd {
	name := a.nickname
	if strings.HasPrefix(text, "/nick ") {
		newNick := chat.Sanitize(strings.TrimSpace(text[6:]))
		if len([]rune(newNick)) > 20 {
			newNick = string([]rune(newNick)[:20])
		}
		if newNick != "" {
			a.nickname = newNick
			chat.SaveConfig(chat.Config{Nickname: newNick})
		}
		return nil
	}
	if text == "/reset" {
		a.nickname = chat.GenerateAnonName()
		chat.SaveConfig(chat.Config{Nickname: a.nickname})
		return nil
	}
	return func() tea.Msg {
		if err := a.chatClient.SendMessage(name, text); err != nil {
			log.Printf("chat send: %v", err)
		}
		return nil
	}
}

func (a *App) switchFocus() {
	a.albumList.Focused = false
	a.trackList.Focused = false
	a.chat.Focused = false

	switch a.focus {
	case focusAlbums:
		a.focus = focusTracks
		a.trackList.Focused = true
	case focusTracks:
		a.focus = focusChat
		a.chat.Focused = true
	case focusChat:
		a.focus = focusAlbums
		a.albumList.Focused = true
	}
}

func (a *App) navigateUp() {
	switch a.focus {
	case focusAlbums:
		a.albumList.Up()
		a.syncTracks()
	case focusTracks:
		a.trackList.Up()
	}
}

func (a *App) navigateDown() {
	switch a.focus {
	case focusAlbums:
		a.albumList.Down()
		a.syncTracks()
	case focusTracks:
		a.trackList.Down()
	}
}

func (a *App) syncTracks() {
	if sel := a.albumList.SelectedAlbum(); sel != nil {
		a.trackList.SetAlbum(sel)
		a.trackList.Color = a.albumList.SelectedColor()
		if a.albumList.Cursor == a.currentAlbumIdx {
			a.trackList.PlayingTrack = a.currentTrackIdx
		}
	}
}

func isQuit(msg tea.KeyMsg) bool {
	k := msg.String()
	return k == "q" || k == "ctrl+c" || k == "esc"
}

func (a *App) renderHelpBar() string {
	t := panels.CurrentTheme()
	br := t.HelpBracket // bracket color
	ky := t.HelpKey     // key color

	lb := t.HelpLabel
	if lb == "" {
		lb = t.TextColor
	}
	key := func(name, label string) string {
		return fmt.Sprintf("\x1b[38;5;%sm[\x1b[38;5;%sm%s\x1b[38;5;%sm] %s\x1b[0m",
			br, ky, name, br, label)
	}

	var shuffleStr, repeatStr string
	if a.controls.Shuffle {
		shuffleStr = fmt.Sprintf("\x1b[38;5;%sm[\x1b[38;5;%smS\x1b[38;5;%sm] \x1b[38;5;%smSHUFFLE\x1b[38;5;231m+\x1b[0m", br, ky, br, lb)
	} else {
		shuffleStr = fmt.Sprintf("\x1b[38;5;%sm[\x1b[38;5;%smS\x1b[38;5;%sm] \x1b[38;5;%smSHUFFLE\x1b[0m", br, ky, br, lb)
	}
	if a.controls.Repeat {
		repeatStr = fmt.Sprintf("\x1b[38;5;%sm[\x1b[38;5;%smR\x1b[38;5;%sm] \x1b[38;5;%smREPEAT\x1b[38;5;231m+\x1b[0m", br, ky, br, lb)
	} else {
		repeatStr = fmt.Sprintf("\x1b[38;5;%sm[\x1b[38;5;%smR\x1b[38;5;%sm] \x1b[38;5;%smREPEAT\x1b[0m", br, ky, br, lb)
	}

	left := " " + strings.Join([]string{
		key("TAB", fmt.Sprintf("\x1b[38;5;%smSWITCH", lb)),
		key("ENTER", fmt.Sprintf("\x1b[38;5;%smPLAY", lb)),
		key("SPACE", a.pauseLabel()),
		fmt.Sprintf("\x1b[38;5;%sm[\x1b[38;5;%sm←\x1b[38;5;%sm/\x1b[38;5;%sm→\x1b[38;5;%sm] \x1b[38;5;%smSEEK\x1b[0m", br, ky, br, ky, br, lb),
		shuffleStr,
		repeatStr,
	}, "  ")

	volChars := []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
	var volStr string
	for i := 0; i < 8; i++ {
		if i < a.controls.Volume*8/10 {
			volStr += fmt.Sprintf("\x1b[38;5;%sm%s", t.TextColor, volChars[i])
		} else {
			volStr += fmt.Sprintf("\x1b[38;5;%sm%s", t.UnplayedColor, volChars[i])
		}
	}
	themeKey := fmt.Sprintf("\x1b[38;5;%sm[\x1b[38;5;%smT\x1b[38;5;%sm] \x1b[38;5;%smTHEME", br, ky, br, lb)
	volKeys := fmt.Sprintf("\x1b[38;5;%sm[\x1b[38;5;%sm-\x1b[38;5;%sm/\x1b[38;5;%sm+\x1b[38;5;%sm]", br, ky, br, ky, br)
	right := fmt.Sprintf("%s %s \x1b[0m%s\x1b[0m ", themeKey, volKeys, volStr)

	leftVis := panels.AnsiVisLen(left)
	rightVis := panels.AnsiVisLen(right)
	gap := a.width - leftVis - rightVis
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (a *App) pauseLabel() string {
	t := panels.CurrentTheme()
	lb := t.HelpLabel
	if lb == "" {
		lb = t.TextColor
	}
	if a.controls.State == panels.StatePaused {
		return fmt.Sprintf("\x1b[38;5;%smRESUME", lb)
	}
	return fmt.Sprintf("\x1b[38;5;%smPAUSE", lb)
}

func (a *App) tickTooSmallVideo(dtMs float64) {
	if a.tsDec == nil {
		return
	}
	a.tsAccum += dtMs / 1000.0
	for a.tsAccum >= a.tsFrameDur {
		a.tsAccum -= a.tsFrameDur
		a.tsFrame++
		if a.tsFrame >= a.tsDec.TotalFrames() {
			a.tsFrame = 0
		}
		a.tsDec.ApplyFrame(a.tsFrame)
	}
}

func (a *App) renderTooSmall() string {
	bright := fmt.Sprintf("\x1b[38;5;%sm", panels.CurrentTheme().ActiveCornerColor)
	yellow := "\x1b[38;5;220m"
	rst := "\x1b[0m"

	// Render video frame — always colorful
	var videoLines []string
	videoW := 0
	if a.tsDec != nil && a.tsRen != nil {
		raw := a.tsRen.Render(a.tsDec, a.tsDec.Width(), a.tsDec.Height(), video.RenderNormal)
		videoLines = strings.Split(raw, "\n")
		videoW = a.tsDec.Width()
	}

	// Content: video + gap + text + gap + sizes
	content := make([]string, 0, len(videoLines)+5)
	for _, line := range videoLines {
		content = append(content, line)
	}
	content = append(content,
		"",
		"",
		bright+"Resize your terminal to continue"+rst,
		"",
		yellow+fmt.Sprintf("%dx%d", a.width, a.height)+rst+
			" \x1b[38;5;231m→ 120x40"+rst,
	)

	// Center vertically
	startY := (a.height - len(content)) / 2
	if startY < 0 {
		startY = 0
	}

	var lines []string
	for y := 0; y < a.height; y++ {
		ci := y - startY
		if ci >= 0 && ci < len(content) {
			line := content[ci]
			visLen := panels.AnsiVisLen(line)
			w := videoW
			if ci >= len(videoLines) {
				w = visLen
			}
			pad := (a.width - w) / 2
			if pad < 0 {
				pad = 0
			}
			lines = append(lines, strings.Repeat(" ", pad)+line)
		} else {
			lines = append(lines, "")
		}
	}
	return a.wrapBg(strings.Join(lines, "\n"))
}

func (a *App) View() string {
	if !a.ready {
		if a.width > 0 && (a.width < 120 || a.height < 40) {
			return a.renderTooSmall()
		}
		return ""
	}

	leftCol := a.video.View() + "\n" + a.chat.View()
	rightCol := a.albumList.View() + "\n" + a.trackList.View()

	topSection := joinHorizontal(leftCol, rightCol, a.video.FrameWidth())
	controlsStr := a.controls.View()

	helpBar := a.renderHelpBar()

	return a.wrapBg(topSection + "\n" + controlsStr + "\n" + helpBar)
}

func joinHorizontal(left, right string, leftWidth int) string {
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")

	maxLines := len(leftLines)
	if len(rightLines) > maxLines {
		maxLines = len(rightLines)
	}

	pad := strings.Repeat(" ", leftWidth)

	var b strings.Builder
	for i := 0; i < maxLines; i++ {
		if i < len(leftLines) {
			line := leftLines[i]
			b.WriteString(line)
			if vis := panels.AnsiVisLen(line); vis < leftWidth {
				b.WriteString(strings.Repeat(" ", leftWidth-vis))
			}
		} else {
			b.WriteString(pad)
		}
		if i < len(rightLines) {
			b.WriteString(rightLines[i])
		}
		if i < maxLines-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func (a *App) wrapBg(content string) string {
	bg := fmt.Sprintf("\x1b[48;5;%sm", panels.CurrentTheme().Bg)

	content = strings.ReplaceAll(content, "\x1b[0m", "\x1b[0m"+bg)
	content = strings.ReplaceAll(content, "\x1b[49m", bg)

	lines := strings.Split(content, "\n")
	var b strings.Builder
	b.WriteString(bg)

	for i := 0; i < a.height; i++ {
		if i < len(lines) {
			line := lines[i]
			visLen := panels.AnsiVisLen(line)
			b.WriteString(line)
			if visLen < a.width {
				b.WriteString(strings.Repeat(" ", a.width-visLen))
			}
		} else {
			b.WriteString(strings.Repeat(" ", a.width))
		}
		if i < a.height-1 {
			b.WriteByte('\n')
		}
	}

	b.WriteString("\x1b[0m")
	return b.String()
}
