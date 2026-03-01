package panels

import (
	"fmt"
	"strings"

	"github.com/dangerous-person/dopogoto/internal/data"
)

var spinFrames = []rune{'▁', '▃', '▅', '▇', '▅', '▃'}

type TrackList struct {
	Album        *data.Album
	Cursor       int
	Offset       int
	PlayingTrack int // -1 if none
	Width        int
	Height       int
	Focused      bool
	Color        string // album color for track text
	AnimTick     int    // animation counter, advanced externally
}

func NewTrackList() TrackList {
	return TrackList{
		PlayingTrack: -1,
	}
}

func (t *TrackList) SetAlbum(album *data.Album) {
	t.Album = album
	t.Cursor = 0
	t.Offset = 0
	t.PlayingTrack = -1
}

func (t *TrackList) Up() {
	if t.Cursor > 0 {
		t.Cursor--
		if t.Cursor < t.Offset {
			t.Offset = t.Cursor
		}
	}
}

func (t *TrackList) Down() {
	if t.Album == nil {
		return
	}
	if t.Cursor < len(t.Album.Tracks)-1 {
		t.Cursor++
		vis := t.visibleTracks()
		if t.Cursor >= t.Offset+vis {
			t.Offset = t.Cursor - vis + 1
		}
	}
}

func (t *TrackList) Top() {
	t.Cursor = 0
	t.Offset = 0
}

func (t *TrackList) Bottom() {
	if t.Album == nil || len(t.Album.Tracks) == 0 {
		return
	}
	t.Cursor = len(t.Album.Tracks) - 1
	vis := t.visibleTracks()
	t.Offset = t.Cursor - vis + 1
	if t.Offset < 0 {
		t.Offset = 0
	}
}

func (t *TrackList) visibleTracks() int {
	n := t.Height - 2 // border (2)
	if n < 1 {
		n = 1
	}
	return n
}

func (t *TrackList) SelectedTrack() *data.Track {
	if t.Album == nil {
		return nil
	}
	if t.Cursor >= 0 && t.Cursor < len(t.Album.Tracks) {
		return &t.Album.Tracks[t.Cursor]
	}
	return nil
}

func (tl TrackList) View() string {
	t := CurrentTheme()

	numColor := t.TextDim
	textColor := t.TextColor

	cornerColor := t.CornerColor
	borderColor := t.BorderColor
	fadeColor := t.FadeColor
	if tl.Focused {
		cornerColor = t.ActiveCornerColor
		borderColor = t.ActiveBorderColor
		fadeColor = t.ActiveFadeColor
	}

	w := tl.Width
	if w < 10 {
		w = 10
	}
	contentW := w - 2 // border (2)
	if contentW < 6 {
		contentW = 6
	}

	var b strings.Builder

	// Top border with title "Songs"
	titleAnsi := BuildTitleGradient("Songs", t.TitleGrad1, t.TitleGrad2, t.TitleGrad3)
	title := fmt.Sprintf(" %s\x1b[38;5;%sm ", titleAnsi, borderColor)
	titleVisLen := 7 // " Songs " = 7 visible chars
	remaining := contentW - titleVisLen
	if remaining < 0 {
		remaining = 0
	}
	leftPad := remaining / 2
	rightPad := remaining - leftPad

	b.WriteString(fmt.Sprintf("\x1b[38;5;%sm╭", cornerColor))
	b.WriteString(FadeBorder(leftPad, FadeDashes, fadeColor, borderColor))
	b.WriteString(title)
	b.WriteString(FadeBorder(rightPad, FadeDashes, fadeColor, borderColor))
	b.WriteString(fmt.Sprintf("\x1b[38;5;%sm╮\x1b[0m\n", cornerColor))

	// Content rows
	vis := tl.visibleTracks()
	contentLines := tl.Height - 2

	lineIdx := 0

	if tl.Album != nil && len(tl.Album.Tracks) > 0 {
		for i := tl.Offset; i < len(tl.Album.Tracks) && i < tl.Offset+vis; i++ {
			track := tl.Album.Tracks[i]

			num := fmt.Sprintf("%02d", i+1)

			maxTitle := contentW - 5 // "  NN " or "  XX " prefix
			titleStr := truncate(track.Title, maxTitle)

			var line string
			isPlaying := i == tl.PlayingTrack
			isSelected := i == tl.Cursor && tl.Focused

			selFg := t.SelectionFg
			if selFg == "" {
				selFg = "231"
			}

			if isPlaying && isSelected {
				s1 := spinFrames[(tl.AnimTick*7)%len(spinFrames)]
				s2 := spinFrames[(tl.AnimTick*7+3)%len(spinFrames)]
				visText := fmt.Sprintf("  %c%c %s", s1, s2, titleStr)
				pad := contentW - len([]rune(visText))
				if pad < 0 {
					pad = 0
				}
				line = fmt.Sprintf("\x1b[48;5;%sm  \x1b[38;5;%sm%c%c \x1b[38;5;%sm%s%s\x1b[0m", t.SelectionBg, selFg, s1, s2, selFg, titleStr, strings.Repeat(" ", pad))
			} else if isPlaying {
				s1 := spinFrames[(tl.AnimTick*7)%len(spinFrames)]
				s2 := spinFrames[(tl.AnimTick*7+3)%len(spinFrames)]
				line = fmt.Sprintf("  \x1b[38;5;%sm%c%c \x1b[38;5;231m%s\x1b[0m", numColor, s1, s2, titleStr)
			} else if isSelected {
				visText := fmt.Sprintf("  %s %s", num, titleStr)
				pad := contentW - len([]rune(visText))
				if pad < 0 {
					pad = 0
				}
				line = fmt.Sprintf("\x1b[48;5;%sm  \x1b[38;5;%sm%s \x1b[38;5;%sm%s%s\x1b[0m", t.SelectionBg, selFg, num, selFg, titleStr, strings.Repeat(" ", pad))
			} else {
				line = fmt.Sprintf("  \x1b[38;5;%sm%s \x1b[38;5;%sm%s\x1b[0m", numColor, num, textColor, titleStr)
			}

			writeBorderedLine(&b, borderColor, fadeColor, line, contentW, lineIdx, contentLines, false)
			lineIdx++
		}
	}

	// Fill remaining rows
	for lineIdx < contentLines {
		writeBorderedLine(&b, borderColor, fadeColor, "", contentW, lineIdx, contentLines, false)
		lineIdx++
	}

	// Bottom border
	b.WriteString(fmt.Sprintf("\x1b[38;5;%sm╰", cornerColor))
	b.WriteString(FadeBorder(contentW, FadeDashes, fadeColor, borderColor))
	b.WriteString(fmt.Sprintf("\x1b[38;5;%sm╯\x1b[0m", cornerColor))

	return b.String()
}
