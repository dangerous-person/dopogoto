package player

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
)

// readSeekCloser wraps bytes.Reader to implement io.ReadSeekCloser.
// io.NopCloser strips the Seeker interface, which go-mp3 needs to compute Length().
type readSeekCloser struct {
	*bytes.Reader
}

func (r readSeekCloser) Close() error { return nil }

// Messages sent to bubbletea
type ProgressMsg struct {
	Position time.Duration
	Length   time.Duration
}

type TrackEndMsg struct{}

type BufferingMsg struct {
	TrackTitle string
}

type TrackStartedMsg struct {
	TrackTitle string
	Duration   time.Duration
}

type ErrorMsg struct {
	Err error
}

type Player struct {
	mu       sync.Mutex
	streamer beep.StreamSeekCloser
	ctrl     *beep.Ctrl
	volume   *effects.Volume
	format   beep.Format

	playing        bool
	vol            float64 // -5.0 to 0.0
	sendMsg        func(interface{})
	stopPoll       chan struct{}
	cancelDownload context.CancelFunc
	initiated      bool
}

func New() *Player {
	return &Player{
		vol: -1.0, // slightly below max
	}
}

// SetSendFunc sets the function used to send messages to bubbletea
func (p *Player) SetSendFunc(fn func(interface{})) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sendMsg = fn
}

// InitSpeaker initializes the speaker (call once)
func (p *Player) InitSpeaker(sampleRate beep.SampleRate) error {
	if p.initiated {
		return nil
	}
	err := speaker.Init(sampleRate, sampleRate.N(time.Second/10))
	if err != nil {
		return fmt.Errorf("speaker init: %w", err)
	}
	p.initiated = true
	return nil
}

// PlayURL downloads an MP3 from a URL and starts playing it.
// NOTE: Caller must set buffering state before calling this (to avoid deadlock
// when called from within bubbletea's Update).
func (p *Player) PlayURL(rawURL, title string) {
	p.Stop()

	// Cancel any in-flight download from a previous call
	p.mu.Lock()
	if p.cancelDownload != nil {
		p.cancelDownload()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancelDownload = cancel
	p.mu.Unlock()

	go func() {
		defer cancel()

		// Encode URL (CDN paths may contain spaces)
		encodedURL := encodeURL(rawURL)

		// Download with 2min timeout, cancellable on next track
		dlCtx, dlCancel := context.WithTimeout(ctx, 120*time.Second)
		defer dlCancel()

		req, err := http.NewRequestWithContext(dlCtx, "GET", encodedURL, nil)
		if err != nil {
			p.sendError(ctx, fmt.Errorf("request: %w", err))
			return
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			p.sendError(ctx, fmt.Errorf("download: %w", err))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			p.sendError(ctx, fmt.Errorf("HTTP %d for %s", resp.StatusCode, rawURL))
			return
		}

		const maxMP3Size = 50 << 20 // 50 MB
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxMP3Size+1))
		if err != nil {
			p.sendError(ctx, fmt.Errorf("read body: %w", err))
			return
		}
		if len(data) > maxMP3Size {
			p.sendError(ctx, fmt.Errorf("file too large (>50 MB): %s", rawURL))
			return
		}

		// Bail if cancelled (user skipped to another track during download)
		if ctx.Err() != nil {
			return
		}

		// Decode MP3
		streamer, format, err := mp3.Decode(readSeekCloser{bytes.NewReader(data)})
		if err != nil {
			p.sendError(ctx, fmt.Errorf("decode mp3: %w", err))
			return
		}

		// Bail if cancelled during decode
		if ctx.Err() != nil {
			streamer.Close()
			return
		}

		p.mu.Lock()

		// Initialize speaker on first track
		if !p.initiated {
			if err := p.InitSpeaker(format.SampleRate); err != nil {
				p.mu.Unlock()
				streamer.Close()
				p.sendError(ctx, err)
				return
			}
		}

		p.streamer = streamer
		p.format = format
		// Audio chain: source -> ctrl -> volume -> speaker
		p.ctrl = &beep.Ctrl{Streamer: streamer}
		p.volume = &effects.Volume{
			Streamer: p.ctrl,
			Base:     2,
			Volume:   p.vol,
		}
		p.playing = true
		p.mu.Unlock()

		duration := format.SampleRate.D(streamer.Len())

		if p.sendMsg != nil {
			p.sendMsg(TrackStartedMsg{
				TrackTitle: title,
				Duration:   duration,
			})
		}

		// Play with callback for track end
		done := make(chan bool)
		speaker.Play(beep.Seq(p.volume, beep.Callback(func() {
			done <- true
		})))

		// Start position polling
		p.mu.Lock()
		p.stopPoll = make(chan struct{})
		stopCh := p.stopPoll // copy ref under mutex for goroutine
		p.mu.Unlock()
		go p.pollPosition(done, stopCh)
	}()
}

// sendError sends an ErrorMsg only if the context hasn't been cancelled.
// Cancelled context means user skipped — not a real error.
func (p *Player) sendError(ctx context.Context, err error) {
	if ctx.Err() != nil {
		return
	}
	if p.sendMsg != nil {
		p.sendMsg(ErrorMsg{Err: err})
	}
}

func (p *Player) pollPosition(done chan bool, stopCh <-chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.mu.Lock()
			if p.streamer != nil && p.format.SampleRate != 0 {
				// Hold p.mu across the speaker.Lock to prevent Stop()
				// from closing the streamer between our unlock and lock.
				speaker.Lock()
				pos := p.format.SampleRate.D(p.streamer.Position())
				length := p.format.SampleRate.D(p.streamer.Len())
				speaker.Unlock()
				p.mu.Unlock()

				if p.sendMsg != nil {
					p.sendMsg(ProgressMsg{Position: pos, Length: length})
				}
			} else {
				p.mu.Unlock()
			}
		case <-done:
			p.mu.Lock()
			p.playing = false
			p.mu.Unlock()
			if p.sendMsg != nil {
				p.sendMsg(TrackEndMsg{})
			}
			return
		case <-stopCh:
			return
		}
	}
}

// Stop stops the current track and cancels any in-flight download.
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cancelDownload != nil {
		p.cancelDownload()
		p.cancelDownload = nil
	}

	if p.stopPoll != nil {
		close(p.stopPoll)
		p.stopPoll = nil
	}

	speaker.Clear()

	if p.streamer != nil {
		p.streamer.Close()
		p.streamer = nil
	}
	p.playing = false
}

// TogglePause toggles pause/resume
func (p *Player) TogglePause() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ctrl == nil {
		return false
	}

	speaker.Lock()
	p.ctrl.Paused = !p.ctrl.Paused
	p.playing = !p.ctrl.Paused
	speaker.Unlock()

	return p.playing
}

// IsPlaying returns whether audio is currently playing
func (p *Player) IsPlaying() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.playing
}

// VolumeUp increases volume
func (p *Player) VolumeUp() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.vol += 0.5
	if p.vol > 0 {
		p.vol = 0
	}
	if p.volume != nil {
		speaker.Lock()
		p.volume.Volume = p.vol
		speaker.Unlock()
	}
	return p.vol
}

// VolumeDown decreases volume
func (p *Player) VolumeDown() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.vol -= 0.5
	if p.vol < -5 {
		p.vol = -5
	}
	if p.volume != nil {
		speaker.Lock()
		p.volume.Volume = p.vol
		speaker.Unlock()
	}
	return p.vol
}

// Volume returns current volume level (0-10 scale for display)
func (p *Player) VolumeLevel() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Map -5..0 to 0..10
	return int((p.vol + 5) * 2)
}

// Seek seeks to a position
func (p *Player) Seek(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.streamer == nil {
		return
	}

	pos := p.format.SampleRate.N(d)
	if pos < 0 {
		pos = 0
	}
	if pos >= p.streamer.Len() {
		pos = p.streamer.Len() - 1
	}

	speaker.Lock()
	p.streamer.Seek(pos)
	speaker.Unlock()
}

// Position returns current playback position
func (p *Player) Position() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.streamer == nil || p.format.SampleRate == 0 {
		return 0
	}

	speaker.Lock()
	pos := p.format.SampleRate.D(p.streamer.Position())
	speaker.Unlock()
	return pos
}

// Close cleans up resources
func (p *Player) Close() {
	p.Stop()
}

// encodeURL properly encodes a URL that may contain spaces in the path
func encodeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	// Encode each path segment
	parts := strings.Split(u.Path, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	u.RawPath = strings.Join(parts, "/")
	return u.String()
}
