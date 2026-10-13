package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type LyricLine struct {
	Start float64
	Text  string
}

type Studio struct {
	win        fyne.Window
	audioPath  string
	outputPath string
	lyrics     *widget.Entry
	style      *widget.Select
	resolution *widget.Select
	status     *widget.Label
	log        *widget.Entry
	renderBtn  *widget.Button
}

var lrcRE = regexp.MustCompile(`^\s*\[(\d{1,2}):(\d{2})(?:[.:](\d{1,3}))?\](.*)$`)
var manualRE = regexp.MustCompile(`^\s*(\d{1,2}):(\d{2})(?:[.:](\d{1,3}))?\s*\|\s*(.*)$`)

func main() {
	a := app.NewWithID("com.ayeshdev.lyricstudio")
	a.Settings().SetTheme(theme.DarkTheme())
	s := &Studio{}
	s.win = a.NewWindow("Lyric Studio — Offline Video Creator")
	s.win.Resize(fyne.NewSize(1000, 760))
	s.buildUI()
	s.win.ShowAndRun()
}

func (s *Studio) buildUI() {
	title := widget.NewLabelWithStyle("LYRIC STUDIO", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Offline lyric video creator  •  Go + Fyne + FFmpeg  •  No AI / no cloud")
	s.status = widget.NewLabel("Choose an audio file to begin.")
	audioLabel := widget.NewLabel("No audio selected")
	audioLabel.Wrapping = fyne.TextWrapBreak
	outputLabel := widget.NewLabel("Output: not selected")
	outputLabel.Wrapping = fyne.TextWrapBreak

	chooseAudio := widget.NewButtonWithIcon("Import audio", theme.FileIcon(), func() {
		d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil {
				s.setStatus(err.Error())
				return
			}
			if r == nil {
				return
			}
			p := r.URI().Path()
			_ = r.Close()
			if strings.HasPrefix(p, "/") && len(p) > 2 && p[2] == ':' {
				p = p[1:]
			}
			s.audioPath = filepath.Clean(p)
			audioLabel.SetText(s.audioPath)
			s.setStatus("Audio selected. Add lyrics, then render.")
		}, s.win)
		d.SetFilter(&audioFilter{})
		d.Show()
	})
	chooseOutput := widget.NewButtonWithIcon("Choose MP4 output", theme.DocumentSaveIcon(), func() {
		d := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
			if err != nil {
				s.setStatus(err.Error())
				return
			}
			if w == nil {
				return
			}
			p := w.URI().Path()
			_ = w.Close()
			if strings.HasPrefix(p, "/") && len(p) > 2 && p[2] == ':' {
				p = p[1:]
			}
			if !strings.HasSuffix(strings.ToLower(p), ".mp4") {
				p += ".mp4"
			}
			s.outputPath = filepath.Clean(p)
			outputLabel.SetText(s.outputPath)
		}, s.win)
		d.SetFileName("lyric-video.mp4")
		d.Show()
	})
	importLRC := widget.NewButtonWithIcon("Import .LRC", theme.FolderOpenIcon(), func() {
		d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil {
				s.setStatus(err.Error())
				return
			}
			if r == nil {
				return
			}
			defer r.Close()
			b, e := readAll(r)
			if e != nil {
				s.setStatus("Read LRC: " + e.Error())
				return
			}
			lines, e := parseLRC(string(b))
			if e != nil {
				s.setStatus(e.Error())
				return
			}
			s.lyrics.SetText(formatTimedLyrics(lines))
			s.setStatus(fmt.Sprintf("Imported %d timestamped lyric lines.", len(lines)))
		}, s.win)
		d.SetFilter(&lrcFilter{})
		d.Show()
	})

	s.lyrics = widget.NewMultiLineEntry()
	s.lyrics.SetPlaceHolder("Paste one lyric line per row for even timing, or use timestamps:\n00:00.000 | First lyric line\n00:04.250 | Next lyric line\n\nYou can also import an .LRC file above.")
	s.lyrics.Wrapping = fyne.TextWrapWord
	s.lyrics.SetMinRowsVisible(12)
	s.style = widget.NewSelect([]string{"Cinematic", "Audio Spectrum"}, func(string) {})
	s.style.SetSelected("Cinematic")
	s.resolution = widget.NewSelect([]string{"1080p · 1920×1080 · 60 FPS", "4K · 3840×2160 · 60 FPS"}, func(string) {})
	s.resolution.SetSelected("1080p · 1920×1080 · 60 FPS")
	s.log = widget.NewMultiLineEntry()
	s.log.SetMinRowsVisible(5)
	s.log.Disable()
	s.renderBtn = widget.NewButtonWithIcon("Render MP4", theme.MediaPlayIcon(), s.render)
	s.renderBtn.Importance = widget.HighImportance

	settings := container.NewGridWithColumns(2,
		container.NewVBox(widget.NewLabel("VISUAL STYLE"), s.style),
		container.NewVBox(widget.NewLabel("OUTPUT QUALITY"), s.resolution),
	)
	files := container.NewVBox(
		container.NewHBox(chooseAudio, chooseOutput, importLRC),
		widget.NewLabel("Audio file"), audioLabel,
		widget.NewLabel("Export destination"), outputLabel,
	)
	lyricsCard := container.NewVBox(widget.NewLabelWithStyle("LYRICS & TIMING", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), s.lyrics,
		widget.NewLabel("Tip: untimed lines are distributed evenly across the track. For precise sync, use LRC or `MM:SS.mmm | lyric` rows."))
	logCard := container.NewVBox(widget.NewLabelWithStyle("RENDER LOG", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), s.log)
	content := container.NewBorder(container.NewVBox(title, subtitle, widget.NewSeparator(), files, widget.NewSeparator(), settings, widget.NewSeparator()),
		container.NewVBox(widget.NewSeparator(), s.status, s.renderBtn), nil, nil,
		container.NewVScroll(container.NewVBox(lyricsCard, widget.NewSeparator(), logCard)))
	s.win.SetContent(container.NewPadded(content))
}

type audioFilter struct{}

func (*audioFilter) Matches(u fyne.URI) bool {
	e := strings.ToLower(filepath.Ext(u.Name()))
	return e == ".mp3" || e == ".wav" || e == ".m4a" || e == ".flac" || e == ".aac" || e == ".ogg" || e == ".opus" || e == ".wma"
}
func (*audioFilter) String() string { return "Audio files" }

type lrcFilter struct{}

func (*lrcFilter) Matches(u fyne.URI) bool {
	return strings.EqualFold(filepath.Ext(u.Name()), ".lrc") || strings.EqualFold(filepath.Ext(u.Name()), ".txt")
}
func (*lrcFilter) String() string { return "LRC or text files" }

func readAll(r fyne.URIReadCloser) ([]byte, error) {
	var b bytes.Buffer
	_, err := b.ReadFrom(r)
	return b.Bytes(), err
}

func parseStamp(mm, ss, frac string) float64 {
	m, _ := strconv.Atoi(mm)
	sec, _ := strconv.Atoi(ss)
	f := 0.0
	if frac != "" {
		if len(frac) == 1 {
			frac += "00"
		} else if len(frac) == 2 {
			frac += "0"
		}
		n, _ := strconv.Atoi(frac)
		f = float64(n) / 1000
	}
	return float64(m*60+sec) + f
}

func parseLRC(raw string) ([]LyricLine, error) {
	var out []LyricLine
	scanner := bufio.NewScanner(strings.NewReader(strings.TrimPrefix(raw, "\ufeff")))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		m := lrcRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		text := strings.TrimSpace(m[4])
		if text == "" {
			continue
		}
		out = append(out, LyricLine{Start: parseStamp(m[1], m[2], m[3]), Text: text})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("No timestamped lyric lines found. Expected [mm:ss.xx]Lyric text.")
	}
	return out, nil
}

func parseManual(raw string) ([]LyricLine, bool, error) {
	var timed, plain []LyricLine
	scanner := bufio.NewScanner(strings.NewReader(raw))
	allTimed := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		m := manualRE.FindStringSubmatch(line)
		if m == nil {
			allTimed = false
			plain = append(plain, LyricLine{Text: line})
			continue
		}
		timed = append(timed, LyricLine{Start: parseStamp(m[1], m[2], m[3]), Text: strings.TrimSpace(m[4])})
	}
	if err := scanner.Err(); err != nil {
		return nil, false, err
	}
	if allTimed && len(timed) > 0 {
		return timed, true, nil
	}
	if len(timed) > 0 && len(plain) > 0 {
		return nil, false, fmt.Errorf("Use either all timestamped rows or plain lyric lines; do not mix both formats.")
	}
	if len(plain) == 0 {
		return nil, false, fmt.Errorf("Enter lyrics or import an LRC file first.")
	}
	return plain, false, nil
}

func formatTimedLyrics(lines []LyricLine) string {
	var b strings.Builder
	for _, l := range lines {
		total := int(l.Start * 1000)
		fmt.Fprintf(&b, "%02d:%02d.%03d | %s\n", total/60000, (total/1000)%60, total%1000, l.Text)
	}
	return strings.TrimSpace(b.String())
}

func (s *Studio) render() {
	if s.audioPath == "" {
		dialog.ShowError(fmt.Errorf("Choose an audio file first."), s.win)
		return
	}
	if s.outputPath == "" {
		dialog.ShowError(fmt.Errorf("Choose an MP4 output destination first."), s.win)
		return
	}
	lines, timed, err := parseManual(s.lyrics.Text)
	if err != nil {
		dialog.ShowError(err, s.win)
		return
	}
	if !fileExists(s.audioPath) {
		dialog.ShowError(fmt.Errorf("Audio file does not exist: %s", s.audioPath), s.win)
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.outputPath), 0755); err != nil {
		dialog.ShowError(err, s.win)
		return
	}
	width, height := 1920, 1080
	if strings.HasPrefix(s.resolution.Selected, "4K") {
		width, height = 3840, 2160
	}
	style := strings.ToLower(s.style.Selected)
	s.renderBtn.Disable()
	s.setStatus("Rendering… this may take a while at 4K/60 FPS.")
	s.appendLog("Starting render…\n")
	go func() {
		defer fyne.Do(func() { s.renderBtn.Enable() })
		if !timed {
			dur, e := audioDuration(s.audioPath)
			if e != nil {
				s.appendLog("ffprobe duration error: " + e.Error() + "\n")
				s.setStatus("Could not read audio duration; check FFmpeg installation.")
				return
			}
			for i := range lines {
				lines[i].Start = dur * float64(i) / float64(len(lines))
			}
		}
		if len(lines) == 0 {
			s.setStatus("No lyrics to render.")
			return
		}
		ass := makeASS(lines, width, height)
		tmp, e := os.CreateTemp("", "lyricstudio-*.ass")
		if e != nil {
			s.appendLog(e.Error() + "\n")
			s.setStatus("Could not create subtitle file.")
			return
		}
		tmpPath := tmp.Name()
		_, e = tmp.WriteString(ass)
		ce := tmp.Close()
		if e == nil {
			e = ce
		}
		defer os.Remove(tmpPath)
		if e != nil {
			s.appendLog(e.Error() + "\n")
			s.setStatus("Could not write subtitle file.")
			return
		}
		args := buildFFmpegArgs(s.audioPath, s.outputPath, tmpPath, style, width, height)
		cmd := exec.Command("ffmpeg", args...)
		cmd.Stderr = logWriter{fn: s.appendLog}
		cmd.Stdout = logWriter{fn: s.appendLog}
		start := time.Now()
		e = cmd.Run()
		if e != nil {
			s.appendLog("\nFFmpeg failed: " + e.Error() + "\n")
			s.setStatus("Render failed. See log; check FFmpeg is on PATH.")
			return
		}
		s.appendLog(fmt.Sprintf("\nCompleted in %s\nSaved: %s\n", time.Since(start).Round(time.Second), s.outputPath))
		s.setStatus("Export complete: " + s.outputPath)
	}()
}

type logWriter struct{ fn func(string) }

func (w logWriter) Write(p []byte) (int, error) { w.fn(string(p)); return len(p), nil }

func audioDuration(path string) (float64, error) {
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	if err != nil {
		return 0, err
	}
	d, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("invalid duration: %q", strings.TrimSpace(string(out)))
	}
	return d, nil
}

func makeASS(lines []LyricLine, w, h int) string {
	fontSize := 72
	marginV := int(float64(h) * 0.19)
	if h >= 2160 {
		fontSize = 144
	}
	var b strings.Builder
	b.WriteString("[Script Info]\nScriptType: v4.00+\nWrapStyle: 2\nScaledBorderAndShadow: yes\nPlayResX: ")
	fmt.Fprintln(&b, w)
	b.WriteString("PlayResY: ")
	fmt.Fprintln(&b, h)
	b.WriteString("\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	fmt.Fprintf(&b, "Style: Default,Arial,%d,&H00FFFFFF,&H004EA1FF,&H00101525,&H99000000,1,0,0,0,100,100,1,0,1,%d,3,2,100,100,%d,1\n", fontSize, maxInt(4, fontSize/18), marginV)
	b.WriteString("\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	for i, l := range lines {
		start := maxFloat(0, l.Start)
		end := start + 3.2
		if i+1 < len(lines) && lines[i+1].Start > start {
			end = lines[i+1].Start
		}
		if end <= start {
			end = start + 1
		}
		fmt.Fprintf(&b, "Dialogue: 0,%s,%s,Default,,0,0,0,,{\\fad(180,220)}%s\n", assTime(start), assTime(end), escapeASS(l.Text))
	}
	return b.String()
}
func assTime(sec float64) string {
	cs := int(sec*100 + 0.5)
	return fmt.Sprintf("%d:%02d:%02d.%02d", cs/360000, (cs/6000)%60, (cs/100)%60, cs%100)
}
func escapeASS(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "{", "\\{")
	s = strings.ReplaceAll(s, "}", "\\}")
	return s
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func buildFFmpegArgs(audio, out, ass, style string, w, h int) []string {
	fps := "60"
	size := fmt.Sprintf("%dx%d", w, h)
	assPath := strings.ReplaceAll(ass, `\`, `/`)
	assPath = strings.ReplaceAll(assPath, ":", `\:`)
	assPath = strings.ReplaceAll(assPath, "'", `\'`)
	var fc string
	if style == "audio spectrum" {
		fc = fmt.Sprintf("[0:a]showspectrum=s=%s:mode=combined:color=intensity:scale=log:slide=scroll:fps=%s,format=yuv420p,subtitles='%s'[v]", size, fps, assPath)
	} else {
		// A dark, subtly animated noise texture gives the cinematic preset depth; a keyed waveform reacts to the audio.
		fc = fmt.Sprintf("color=c=0x070B18:s=%s:r=%s[bg];[bg]noise=alls=7:allf=t+u,format=yuv420p[base];[0:a]showwaves=s=%dx%d:mode=line:rate=%s:colors=0x4EA1FF,format=rgba,colorkey=black:0.08:0.0[wave];[base][wave]overlay=0:%d:shortest=1,subtitles='%s'[v]", size, fps, w, int(float64(h)*0.19), fps, int(float64(h)*0.67), assPath)
	}
	return []string{"-hide_banner", "-y", "-i", audio, "-filter_complex", fc, "-map", "[v]", "-map", "0:a:0", "-r", fps, "-c:v", "libx264", "-preset", "slow", "-crf", crfFor(w), "-pix_fmt", "yuv420p", "-c:a", "aac", "-b:a", "320k", "-movflags", "+faststart", out}
}
func crfFor(w int) string {
	if w >= 3840 {
		return "17"
	}
	return "16"
}
func fileExists(p string) bool       { st, e := os.Stat(p); return e == nil && !st.IsDir() }
func (s *Studio) setStatus(t string) { fyne.Do(func() { s.status.SetText(t) }) }
func (s *Studio) appendLog(t string) {
	fyne.Do(func() { s.log.SetText(s.log.Text + t); s.log.Refresh() })
}
