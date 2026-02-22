package panels

import (
	"fmt"
	"strings"

	"github.com/dangerous-person/dopogoto/internal/chat"
)

// localMsg is a local-only message anchored to a position in the remote stream.
type localMsg struct {
	msg         chat.Message
	afterRemote int // number of remote messages that existed when this was added
}

// Chat is the chat panel with message history and text input.
type Chat struct {
	Width   int
	Height  int
	Focused bool
	remote  []chat.Message // from server (replaced on each refresh)
	local   []localMsg     // local-only, anchored (survive refreshes)
	input   string
	offline bool
	scroll  int // scroll offset from bottom (0 = showing latest)
}

func NewChat() Chat {
	return Chat{}
}

// allMessages merges remote and local messages in chronological order.
// Local messages are inserted after the remote messages that existed when they were added.
func (c *Chat) allMessages() []chat.Message {
	if len(c.local) == 0 {
		return c.remote
	}
	msgs := make([]chat.Message, 0, len(c.remote)+len(c.local))
	li := 0 // index into c.local
	for ri, rm := range c.remote {
		// Insert any local messages anchored at this position
		for li < len(c.local) && c.local[li].afterRemote == ri {
			msgs = append(msgs, c.local[li].msg)
			li++
		}
		msgs = append(msgs, rm)
	}
	// Remaining locals anchored at or beyond end of remote
	for li < len(c.local) {
		msgs = append(msgs, c.local[li].msg)
		li++
	}
	return msgs
}

// SetMessages replaces the remote message list. Local messages are preserved.
func (c *Chat) SetMessages(msgs []chat.Message) {
	c.remote = msgs
	c.scroll = 0
}

// AddLocalMessage inserts a local-only message (not sent to server).
// Anchored after the current remote messages so later remotes appear after it.
func (c *Chat) AddLocalMessage(name, text string) {
	c.local = append(c.local, localMsg{
		msg:         chat.Message{Name: name, Text: text},
		afterRemote: len(c.remote),
	})
}

// SetOffline marks the chat as offline.
func (c *Chat) SetOffline(offline bool) {
	c.offline = offline
}

// InsertRune adds a character to the input (max 280 chars).
func (c *Chat) InsertRune(r rune) {
	if len([]rune(c.input)) >= 280 {
		return
	}
	c.input += string(r)
}

// Backspace removes the last character from input.
func (c *Chat) Backspace() {
	if len(c.input) > 0 {
		runes := []rune(c.input)
		c.input = string(runes[:len(runes)-1])
	}
}

// ClearInput clears the input.
func (c *Chat) ClearInput() string {
	text := c.input
	c.input = ""
	return text
}

// ScrollUp scrolls chat history up.
func (c *Chat) ScrollUp() {
	total := len(c.remote) + len(c.local)
	maxScroll := total - c.viewableLines()
	if maxScroll < 0 {
		maxScroll = 0
	}
	c.scroll++
	if c.scroll > maxScroll {
		c.scroll = maxScroll
	}
}

// ScrollDown scrolls chat history down.
func (c *Chat) ScrollDown() {
	c.scroll--
	if c.scroll < 0 {
		c.scroll = 0
	}
}

func (c Chat) viewableLines() int {
	h := c.Height - 4 // border (2) + input separator (1) + input line (1)
	if h < 1 {
		h = 1
	}
	return h
}

func (c Chat) View() string {
	t := CurrentTheme()

	cornerColor := t.CornerColor
	borderColor := t.BorderColor
	fadeColor := t.FadeColor
	if c.Focused {
		cornerColor = t.ActiveCornerColor
		borderColor = t.ActiveBorderColor
		fadeColor = t.ActiveFadeColor
	}

	w := c.Width
	if w < 10 {
		w = 10
	}
	contentW := w - 4 // border (2) + padding (1 each side)
	if contentW < 6 {
		contentW = 6
	}

	var b strings.Builder

	// Top border with title "Chat"
	titleText := "Chat"
	if c.offline {
		titleText = "Chat Offline"
	}
	titleAnsi := BuildTitleGradient(titleText, t.TitleGrad1, t.TitleGrad2, t.TitleGrad3)
	title := fmt.Sprintf(" %s\x1b[38;5;%sm ", titleAnsi, borderColor)
	titleVisLen := len([]rune(titleText)) + 2 // spaces around title
	remaining := contentW + 2 - titleVisLen
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

	// Message area
	viewLines := c.viewableLines()
	lines := c.renderMessages(contentW)
	contentLines := viewLines + 2 // messages + separator + input

	// Apply scroll
	start := len(lines) - viewLines - c.scroll
	if start < 0 {
		start = 0
	}
	end := start + viewLines
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[start:end]

	for i := 0; i < viewLines; i++ {
		sideColor := borderColor
		if i < FadeDashes || i >= contentLines-FadeDashes {
			sideColor = fadeColor
		}
		b.WriteString(fmt.Sprintf("\x1b[38;5;%sm│\x1b[0m ", sideColor))
		if i < len(visible) {
			line := visible[i]
			visLen := AnsiVisLen(line)
			pad := contentW - visLen
			if pad < 0 {
				pad = 0
			}
			b.WriteString(line)
			b.WriteString(strings.Repeat(" ", pad))
		} else {
			b.WriteString(strings.Repeat(" ", contentW))
		}
		b.WriteString(fmt.Sprintf(" \x1b[38;5;%sm│\x1b[0m\n", sideColor))
	}

	// Input line separator
	sepRow := viewLines
	sepColor := borderColor
	if sepRow < FadeDashes || sepRow >= contentLines-FadeDashes {
		sepColor = fadeColor
	}
	b.WriteString(fmt.Sprintf("\x1b[38;5;%sm├", sepColor))
	b.WriteString(FadeBorder(contentW+2, FadeDashes, fadeColor, borderColor))
	b.WriteString(fmt.Sprintf("\x1b[38;5;%sm┤\x1b[0m\n", sepColor))

	// Input line
	inputRow := viewLines + 1
	inputSideColor := borderColor
	if inputRow < FadeDashes || inputRow >= contentLines-FadeDashes {
		inputSideColor = fadeColor
	}

	var inputContent string
	var inputVisLen int
	if c.Focused {
		inputDisplay := c.input
		maxInput := contentW - 1 // leave room for cursor
		if len([]rune(inputDisplay)) > maxInput {
			inputDisplay = string([]rune(inputDisplay)[len([]rune(inputDisplay))-maxInput:])
		}
		inputVisLen = len([]rune(inputDisplay)) + 1 // +1 for cursor
		inputColor := t.ChatInputColor
		if inputColor == "" {
			inputColor = t.ChatNameColor
		}
		inputContent = fmt.Sprintf("\x1b[38;5;%sm%s\x1b[25m\x1b[38;5;%sm█\x1b[0m", t.TextColor, inputDisplay, inputColor)
	} else {
		placeholder := "> type a message..."
		if len([]rune(placeholder)) > contentW {
			placeholder = string([]rune(placeholder)[:contentW])
		}
		inputVisLen = len([]rune(placeholder))
		inputContent = fmt.Sprintf("\x1b[38;5;%sm%s\x1b[0m", t.TextDim, placeholder)
	}
	inputPad := contentW - inputVisLen
	if inputPad < 0 {
		inputPad = 0
	}
	b.WriteString(fmt.Sprintf("\x1b[38;5;%sm│\x1b[0m %s%s \x1b[38;5;%sm│\x1b[0m\n",
		inputSideColor, inputContent, strings.Repeat(" ", inputPad), inputSideColor))

	// Bottom border
	b.WriteString(fmt.Sprintf("\x1b[38;5;%sm╰", cornerColor))
	b.WriteString(FadeBorder(contentW+2, FadeDashes, fadeColor, borderColor))
	b.WriteString(fmt.Sprintf("\x1b[38;5;%sm╯\x1b[0m", cornerColor))

	return b.String()
}

func (c Chat) renderMessages(maxW int) []string {
	t := CurrentTheme()

	if c.offline {
		return []string{
			fmt.Sprintf("\x1b[38;5;%smCHAT OFFLINE\x1b[0m", t.ChatOffline),
			fmt.Sprintf("\x1b[38;5;%smNo internet connection\x1b[0m", t.ChatOffline),
		}
	}

	all := c.allMessages()
	if len(all) == 0 {
		return []string{
			fmt.Sprintf("\x1b[38;5;%smNo messages yet\x1b[0m", t.ChatOffline),
		}
	}

	var lines []string
	for _, msg := range all {
		nameColor := t.ChatNameColor
		textColor := t.TextColor

		name := chat.Sanitize(msg.Name)
		isSystem := name == "[system]"
		isUpdate := name == "[update]"
		if isSystem {
			name = "(◕‿◕)"
			nameColor = t.TextDim
			textColor = t.TextDim
		}
		if isUpdate {
			name = "(◕‿◕)"
			nameColor = t.SelectionBg
			textColor = t.SelectionBg
			isSystem = true // use system message formatting
		}
		// Clamp name to 16 runes so a long remote name can't push text off-screen
		if nameRunes := []rune(name); len(nameRunes) > 16 {
			name = string(nameRunes[:16])
		}
		text := chat.Sanitize(msg.Text)

		var prefix string
		var prefixLen int
		if isSystem {
			prefix = fmt.Sprintf("\x1b[38;5;%sm%s \x1b[0m", nameColor, name)
			prefixLen = len([]rune(name)) + 1 // "name "
		} else {
			prefix = fmt.Sprintf("\x1b[38;5;%sm%s:\x1b[0m ", nameColor, name)
			prefixLen = len([]rune(name)) + 2 // "name: "
		}

		available := maxW - prefixLen
		if available < 1 {
			available = 1
		}
		if available > maxW {
			available = maxW
		}

		textRunes := []rune(text)

		if len(textRunes) <= available {
			lines = append(lines, fmt.Sprintf("%s\x1b[38;5;%sm%s\x1b[0m", prefix, textColor, text))
		} else {
			lines = append(lines, fmt.Sprintf("%s\x1b[38;5;%sm%s\x1b[0m", prefix, textColor, string(textRunes[:available])))
			remaining := textRunes[available:]
			indent := strings.Repeat(" ", prefixLen)
			for len(remaining) > 0 {
				end := available
				if end > len(remaining) {
					end = len(remaining)
				}
				lines = append(lines, fmt.Sprintf("%s\x1b[38;5;%sm%s\x1b[0m", indent, textColor, string(remaining[:end])))
				remaining = remaining[end:]
			}
		}
	}

	return lines
}
