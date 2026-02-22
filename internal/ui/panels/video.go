package panels

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/dangerous-person/dopogoto/internal/video"
)

// Video is a bubbletea component that plays looping ASCII videos in random order.
type Video struct {
	clips     []*video.Decoder
	renderers []*video.Renderer
	current   int // index into clips
	Width     int
	Height    int

	frame     int
	tickAccum float64
	frameDur  float64
	order     []int
	orderIdx  int
}

// NewVideo creates a video panel from multiple brotli/gzip video data blobs.
func NewVideo(allData ...[]byte) (Video, error) {
	var clips []*video.Decoder
	var renderers []*video.Renderer

	for _, data := range allData {
		dec, err := video.NewDecoder(data)
		if err != nil {
			return Video{}, err
		}
		clips = append(clips, dec)
		renderers = append(renderers, video.NewRenderer(dec.Data.Palette))
	}

	if len(clips) == 0 {
		return Video{}, fmt.Errorf("no video clips provided")
	}

	v := Video{
		clips:     clips,
		renderers: renderers,
	}
	v.current = 0
	v.shuffle()
	v.startClip()

	return v, nil
}

func (v *Video) shuffle() {
	v.order = rand.Perm(len(v.clips))
	v.orderIdx = 0
}

func (v *Video) pickClip() {
	if v.orderIdx >= len(v.order) {
		v.shuffle()
	}
	v.current = v.order[v.orderIdx]
	v.orderIdx++

	v.frame = 0
	v.tickAccum = 0

	dec := v.clips[v.current]
	fps := dec.FPS()
	if fps <= 0 {
		fps = 30
	}
	v.frameDur = 1.0 / float64(fps)
	dec.ApplyFrame(0)
}

func (v *Video) startClip() {
	v.frame = 0
	v.tickAccum = 0

	dec := v.clips[v.current]
	fps := dec.FPS()
	if fps <= 0 {
		fps = 30
	}
	v.frameDur = 1.0 / float64(fps)
	dec.ApplyFrame(0)
}

// NextClip advances to the next random video clip.
func (v *Video) NextClip() {
	v.pickClip()
}

// Tick advances the video by dt milliseconds.
func (v *Video) Tick(dtMs float64) {
	if len(v.clips) == 0 {
		return
	}

	dec := v.clips[v.current]
	v.tickAccum += dtMs / 1000.0

	for v.tickAccum >= v.frameDur {
		v.tickAccum -= v.frameDur
		v.frame++
		if v.frame >= dec.TotalFrames() {
			v.NextClip()
			return
		}
		v.clips[v.current].ApplyFrame(v.frame)
	}
}

// View renders the current frame as an ANSI string wrapped in a border.
func (v Video) View() string {
	if len(v.clips) == 0 {
		return ""
	}

	t := CurrentTheme()
	dec := v.clips[v.current]
	ren := v.renderers[v.current]

	contentW := v.Width - 2
	contentH := v.Height - 2
	if contentW < 1 || contentH < 1 {
		return ""
	}

	mode := video.RenderNormal
	if t.Name == "Mono" {
		mode = video.RenderGrayscale
	} else if t.VideoTintHue > 0 {
		ren.SetTint(t.VideoTintHue, t.VideoTintSat)
		mode = video.RenderTint
	}
	raw := ren.Render(dec, contentW, contentH, mode)
	lines := strings.Split(raw, "\n")

	var b strings.Builder

	// Build "You're listening to Dopo Goto" with gradient on capital letters
	title := fmt.Sprintf(" \x1b[38;5;%smY\x1b[38;5;%smo\x1b[38;5;%smu're listening to \x1b[38;5;%smD\x1b[38;5;%smo\x1b[38;5;%smpo \x1b[38;5;%smG\x1b[38;5;%smo\x1b[38;5;%smto\x1b[0m\x1b[38;5;%sm ",
		t.TitleGrad1, t.TitleGrad2, t.TitleGrad3,
		t.TitleGrad1, t.TitleGrad2, t.TitleGrad3,
		t.TitleGrad1, t.TitleGrad2, t.TitleGrad3,
		t.BorderColor)
	titleVisLen := 31
	remaining := contentW - titleVisLen
	if remaining < 0 {
		remaining = 0
	}
	leftPad := remaining / 2
	rightPad := remaining - leftPad

	b.WriteString(fmt.Sprintf("\x1b[38;5;%sm╭", t.CornerColor))
	b.WriteString(FadeBorder(leftPad, FadeDashes, t.FadeColor, t.BorderColor))
	b.WriteString(title)
	b.WriteString(FadeBorder(rightPad, FadeDashes, t.FadeColor, t.BorderColor))
	b.WriteString(fmt.Sprintf("\x1b[38;5;%sm╮\x1b[0m\n", t.CornerColor))

	for i := 0; i < contentH; i++ {
		sideColor := t.BorderColor
		if i < FadeDashes || i >= contentH-FadeDashes {
			sideColor = t.FadeColor
		}
		b.WriteString(fmt.Sprintf("\x1b[38;5;%sm│\x1b[0m", sideColor))
		if i < len(lines) {
			line := lines[i]
			vis := AnsiVisLen(line)
			b.WriteString(line)
			if vis < contentW {
				b.WriteString(strings.Repeat(" ", contentW-vis))
			}
		} else {
			b.WriteString(strings.Repeat(" ", contentW))
		}
		b.WriteString(fmt.Sprintf("\x1b[38;5;%sm│\x1b[0m\n", sideColor))
	}

	b.WriteString(fmt.Sprintf("\x1b[38;5;%sm╰", t.CornerColor))
	b.WriteString(FadeBorder(contentW, FadeDashes, t.FadeColor, t.BorderColor))
	b.WriteString(fmt.Sprintf("\x1b[38;5;%sm╯\x1b[0m", t.CornerColor))

	return b.String()
}

// FrameWidth returns the total width of the video panel including border.
func (v Video) FrameWidth() int {
	if len(v.clips) == 0 {
		return 0
	}
	return v.clips[v.current].Width() + 2
}

// VideoWidth returns the native video width.
func (v Video) VideoWidth() int {
	if len(v.clips) == 0 {
		return 0
	}
	return v.clips[v.current].Width()
}

// VideoHeight returns the native video height.
func (v Video) VideoHeight() int {
	if len(v.clips) == 0 {
		return 0
	}
	return v.clips[v.current].Height()
}
