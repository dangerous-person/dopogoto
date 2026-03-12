package panels

import (
	"fmt"
	"strings"

	"github.com/dangerous-person/dopogoto/internal/data"
)

type AlbumList struct {
	Albums  []data.Album
	Cursor  int
	Offset  int
	Width   int
	Height  int
	Focused bool
}

func NewAlbumList(albums []data.Album) AlbumList {
	return AlbumList{
		Albums: albums,
		Width:  35,
		Height: 20,
	}
}

func (a *AlbumList) Up() {
	if a.Cursor > 0 {
		a.Cursor--
		if a.Cursor < a.Offset {
			a.Offset = a.Cursor
		}
	}
}

func (a *AlbumList) Down() {
	if a.Cursor < len(a.Albums)-1 {
		a.Cursor++
		vis := a.visibleAlbums()
		if a.Cursor >= a.Offset+vis {
			a.Offset = a.Cursor - vis + 1
		}
	}
}

func (a *AlbumList) Top() {
	a.Cursor = 0
	a.Offset = 0
}

func (a *AlbumList) Bottom() {
	if len(a.Albums) == 0 {
		return
	}
	a.Cursor = len(a.Albums) - 1
	vis := a.visibleAlbums()
	a.Offset = a.Cursor - vis + 1
	if a.Offset < 0 {
		a.Offset = 0
	}
}

// visibleAlbums returns how many albums fit in the panel.
func (a *AlbumList) visibleAlbums() int {
	n := a.Height - 2 // border (2)
	if n < 1 {
		n = 1
	}
	return n
}

func (a *AlbumList) SelectedAlbum() *data.Album {
	if a.Cursor >= 0 && a.Cursor < len(a.Albums) {
		return &a.Albums[a.Cursor]
	}
	return nil
}

// SelectedColor returns the color for the currently selected album.
func (a *AlbumList) SelectedColor() string {
	t := CurrentTheme()
	if a.Cursor >= 0 && a.Cursor < len(t.AlbumColors) {
		return t.AlbumColors[a.Cursor]
	}
	return t.TextColor
}

func (a AlbumList) View() string {
	t := CurrentTheme()

	cornerColor := t.CornerColor
	borderColor := t.BorderColor
	fadeColor := t.FadeColor
	if a.Focused {
		cornerColor = t.ActiveCornerColor
		borderColor = t.ActiveBorderColor
		fadeColor = t.ActiveFadeColor
	}

	w := a.Width
	if w < 10 {
		w = 10
	}
	contentW := w - 2 // border (2)
	if contentW < 6 {
		contentW = 6
	}

	var b strings.Builder

	// Top border with title: ╭─ Albums ──...──╮
	// 1st letter white, 2nd light yellow, rest yellow
	titleAnsi := BuildTitleGradient("Albums", t.TitleGrad1, t.TitleGrad2, t.TitleGrad3)
	title := fmt.Sprintf(" %s\x1b[38;5;%sm ", titleAnsi, borderColor)
	titleVisLen := 8 // " Albums " = 8 visible chars
	remaining := contentW - titleVisLen
	if remaining < 0 {
		remaining = 0
	}
	leftPad := remaining / 2
	rightPad := remaining - leftPad
	if leftPad < 0 {
		leftPad = 0
	}
	if rightPad < 0 {
		rightPad = 0
	}
	b.WriteString(fmt.Sprintf("\x1b[38;5;%sm╭", cornerColor))
	b.WriteString(FadeBorder(leftPad, FadeDashes, fadeColor, borderColor))
	b.WriteString(title)
	b.WriteString(FadeBorder(rightPad, FadeDashes, fadeColor, borderColor))
	b.WriteString(fmt.Sprintf("\x1b[38;5;%sm╮\x1b[0m\n", cornerColor))

	// Content rows
	vis := a.visibleAlbums()
	contentLines := a.Height - 2 // total rows inside borders

	lineIdx := 0
	for i := a.Offset; i < len(a.Albums) && i < a.Offset+vis; i++ {
		album := a.Albums[i]
		title := truncate(album.Title, contentW-3) // 2 prefix + 1 margin

		if i == a.Cursor {
			// Active album: text on selection bg, full row
			selFg := t.SelectionFg
			if selFg == "" {
				selFg = "231"
			}
			visText := "  " + title
			pad := contentW - len([]rune(visText))
			if pad < 0 {
				pad = 0
			}
			titleLine := fmt.Sprintf("\x1b[48;5;%sm\x1b[38;5;%sm%s%s\x1b[0m", t.SelectionBg, selFg, visText, strings.Repeat(" ", pad))
			writeBorderedLine(&b, borderColor, fadeColor, titleLine, contentW, lineIdx, contentLines, false)
		} else {
			titleLine := fmt.Sprintf("  \x1b[38;5;%sm%s\x1b[0m", t.TextColor, title)
			writeBorderedLine(&b, borderColor, fadeColor, titleLine, contentW, lineIdx, contentLines, false)
		}
		lineIdx++
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
