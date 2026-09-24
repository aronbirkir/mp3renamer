package main

import (
	"errors"
	"fmt"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bogem/id3v2/v2"
	"github.com/ncruces/zenity"
)

const defaultPattern = "{bpm:03} {key:03} {artist} - {title}"

type FileRename struct {
	originalName string
	newName      string
}

type statusKind int

const (
	statusInfo statusKind = iota
	statusSuccess
	statusError
)

func (k statusKind) color() color.NRGBA {
	switch k {
	case statusSuccess:
		return pal.Success
	case statusError:
		return pal.Error
	default:
		return pal.Muted
	}
}

type UI struct {
	window       *app.Window
	folder       widget.Editor
	pattern      widget.Editor
	browseButton widget.Clickable
	scanButton   widget.Clickable
	applyButton  widget.Clickable
	files        []FileRename
	scanned      bool
	list         widget.List

	status     string
	statusKind statusKind

	themeMode    ThemeMode
	themeButtons [3]widget.Clickable
	sysTheme     systemTheme

	// picked receives folders chosen in the native dialog, which runs off the UI goroutine.
	picked   chan string
	browsing bool
}

func main() {
	go func() {
		window := new(app.Window)
		window.Option(
			app.Title("MP3 Renamer"),
			app.Size(unit.Dp(960), unit.Dp(680)),
			app.MinSize(unit.Dp(640), unit.Dp(420)),
		)
		err := run(window)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	setAppIcon()
	app.Main()
}

func newTheme() *material.Theme {
	th := material.NewTheme()
	th.TextSize = unit.Sp(15)
	return th
}

// applyTheme picks the palette for this frame from the user's preference.
func (ui *UI) applyTheme(th *material.Theme) {
	dark := ui.themeMode == ThemeDark ||
		(ui.themeMode != ThemeLight && ui.sysTheme.dark.Load())
	if dark {
		pal = &darkPalette
	} else {
		pal = &lightPalette
	}
	th.Palette = material.Palette{
		Bg:         pal.Bg,
		Fg:         pal.Fg,
		ContrastBg: pal.Accent,
		ContrastFg: rgb(0xffffff),
	}
}

func run(window *app.Window) error {
	theme := newTheme()
	var ops op.Ops
	ui := &UI{
		window:  window,
		folder:  widget.Editor{SingleLine: true, Submit: true},
		pattern: widget.Editor{SingleLine: true, Submit: true},
		list:    widget.List{List: layout.List{Axis: layout.Vertical}},
		picked:  make(chan string, 1),
	}

	cfg := loadConfig()
	if cfg.Pattern == "" {
		cfg.Pattern = defaultPattern
	}
	ui.pattern.SetText(cfg.Pattern)
	ui.themeMode = cfg.Theme
	if ui.themeMode == "" {
		ui.themeMode = ThemeSystem
	}
	ui.sysTheme.watch(window)
	if cfg.Folder != "" {
		ui.folder.SetText(cfg.Folder)
		ui.scanFiles()
	}

	for {
		switch e := window.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			ui.update(gtx)
			ui.applyTheme(theme)
			ui.Layout(gtx, theme)
			e.Frame(gtx.Ops)
		}
	}
}

func (ui *UI) update(gtx C) {
	select {
	case dir := <-ui.picked:
		ui.browsing = false
		if dir != "" {
			ui.folder.SetText(dir)
			ui.scanFiles()
		}
	default:
	}

	for i := range ui.themeButtons {
		if ui.themeButtons[i].Clicked(gtx) && ui.themeMode != themeModes[i] {
			ui.themeMode = themeModes[i]
			ui.saveConfig()
		}
	}
	if ui.browseButton.Clicked(gtx) && !ui.browsing {
		ui.browse()
	}
	if ui.scanButton.Clicked(gtx) {
		ui.scanFiles()
	}
	if ui.applyButton.Clicked(gtx) && len(ui.files) > 0 {
		ui.applyRenames()
	}
	for _, ed := range []*widget.Editor{&ui.folder, &ui.pattern} {
		for {
			ev, ok := ed.Update(gtx)
			if !ok {
				break
			}
			if _, ok := ev.(widget.SubmitEvent); ok {
				ui.scanFiles()
			}
		}
	}
}

// browse opens the native folder picker without blocking the UI.
func (ui *UI) browse() {
	ui.browsing = true
	start := ui.folder.Text()
	go func() {
		opts := []zenity.Option{zenity.Directory(), zenity.Title("Choose music folder")}
		if start != "" {
			opts = append(opts, zenity.Filename(start+string(filepath.Separator)))
		}
		dir, err := zenity.SelectFile(opts...)
		if err != nil && !errors.Is(err, zenity.ErrCanceled) {
			log.Printf("folder dialog: %v", err)
		}
		ui.picked <- dir
		ui.window.Invalidate()
	}()
}

func (ui *UI) setStatus(k statusKind, format string, args ...any) {
	ui.status = fmt.Sprintf(format, args...)
	ui.statusKind = k
}

func (ui *UI) saveConfig() {
	cfg := Config{Folder: ui.folder.Text(), Pattern: ui.pattern.Text(), Theme: ui.themeMode}
	if err := saveConfig(cfg); err != nil {
		log.Printf("saving config: %v", err)
	}
}

func (ui *UI) scanFiles() {
	ui.files = nil
	ui.scanned = false
	path := strings.TrimSpace(ui.folder.Text())
	if path == "" {
		ui.setStatus(statusInfo, "Choose a folder to get started.")
		return
	}
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		ui.setStatus(statusError, "Can't read folder: %v", err)
		return
	}
	ui.scanned = true
	ui.saveConfig()

	total, failed := 0, 0
	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() || !strings.EqualFold(filepath.Ext(dirEntry.Name()), ".mp3") {
			continue
		}
		total++
		fileName := dirEntry.Name()
		m, err := id3v2.Open(filepath.Join(path, fileName), id3v2.Options{Parse: true})
		if err != nil {
			failed++
			continue
		}
		newName := ui.generateNewName(m)
		m.Close()
		if newName != fileName {
			ui.files = append(ui.files, FileRename{
				originalName: fileName,
				newName:      newName,
			})
		}
	}

	switch {
	case total == 0:
		ui.setStatus(statusInfo, "No MP3 files in this folder.")
	case len(ui.files) == 0:
		ui.setStatus(statusSuccess, "All %d files already match the pattern.", total)
	default:
		ui.setStatus(statusInfo, "%d of %d files will be renamed.", len(ui.files), total)
	}
	if failed > 0 {
		ui.status += fmt.Sprintf(" %d could not be read.", failed)
		ui.statusKind = statusError
	}
}

func (ui *UI) generateNewName(m *id3v2.Tag) string {
	pattern := ui.pattern.Text()
	songInfo := map[string]string{
		"artist": strings.Title(strings.ToLower(m.Artist())),
		"title":  strings.Title(strings.ToLower(m.Title())),
		"bpm":    m.GetTextFrame("TBPM").Text,
		"key":    m.GetTextFrame("TKEY").Text,
	}

	for key, value := range songInfo {
		token := "{" + key
		if strings.Contains(pattern, token) {
			start := strings.Index(pattern, token)
			end := strings.Index(pattern[start:], "}") + start
			if end > start {
				format := pattern[start+len(token) : end]
				if format != "" {
					format = strings.Trim(format, ":")
					if pad, err := strconv.Atoi(format); err == nil {
						value = fmt.Sprintf("%0"+strconv.Itoa(pad)+"s", value)
					}
				}
				pattern = pattern[:start] + value + pattern[end+1:]
			}
		}
	}

	return pattern + ".mp3"
}

func (ui *UI) applyRenames() {
	path := ui.folder.Text()
	renamed, failed := 0, 0
	for _, file := range ui.files {
		oldPath := filepath.Join(path, file.originalName)
		newPath := filepath.Join(path, file.newName)
		if oldPath == newPath {
			continue
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			log.Printf("rename %s: %v", file.originalName, err)
			failed++
			continue
		}
		renamed++
	}
	ui.scanFiles() // Refresh the list
	if failed > 0 {
		ui.setStatus(statusError, "Renamed %d files, %d failed.", renamed, failed)
	} else {
		ui.setStatus(statusSuccess, "Renamed %d files.", renamed)
	}
}

// ---- Layout ----

func (ui *UI) Layout(gtx C, th *material.Theme) D {
	fill(gtx, pal.Bg)
	return layout.UniformInset(unit.Dp(24)).Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return ui.layoutHeader(gtx, th) }),
			vspace(20),
			layout.Rigid(func(gtx C) D { return ui.layoutSettings(gtx, th) }),
			vspace(16),
			layout.Flexed(1, func(gtx C) D { return ui.layoutResults(gtx, th) }),
		)
	})
}

func (ui *UI) layoutHeader(gtx C, th *material.Theme) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					l := material.H5(th, "MP3 Renamer")
					l.Font.Weight = font.Bold
					return l.Layout(gtx)
				}),
				vspace(2),
				layout.Rigid(func(gtx C) D {
					l := material.Body2(th, "Rename tracks from their ID3 tags")
					l.Color = pal.Muted
					return l.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(func(gtx C) D { return ui.layoutThemeSwitch(gtx, th) }),
	)
}

// layoutThemeSwitch draws a segmented System / Light / Dark control.
func (ui *UI) layoutThemeSwitch(gtx C, th *material.Theme) D {
	segments := make([]layout.FlexChild, len(themeModes))
	for i, mode := range themeModes {
		segments[i] = layout.Rigid(func(gtx C) D {
			selected := ui.themeMode == mode
			return material.Clickable(gtx, &ui.themeButtons[i], func(gtx C) D {
				bg := pal.Surface
				if selected {
					bg = pal.Accent
				}
				return layout.Background{}.Layout(gtx, rounded(bg, 6), func(gtx C) D {
					return layout.Inset{Top: 9, Bottom: 3, Left: 12, Right: 12}.Layout(gtx, func(gtx C) D {
						l := material.Body2(th, mode.Label())
						l.TextSize = unit.Sp(13)
						l.Font.Weight = font.SemiBold
						l.Color = pal.Muted
						if selected {
							l.Color = rgb(0xffffff)
						}
						return l.Layout(gtx)
					})
				})
			})
		})
	}
	return widget.Border{Color: pal.Border, CornerRadius: unit.Dp(9), Width: unit.Dp(1)}.Layout(gtx, func(gtx C) D {
		return layout.Background{}.Layout(gtx, rounded(pal.Surface, 9), func(gtx C) D {
			return layout.UniformInset(unit.Dp(3)).Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx, segments...)
			})
		})
	})
}

func card(gtx C, w layout.Widget) D {
	return widget.Border{Color: pal.Border, CornerRadius: unit.Dp(12), Width: unit.Dp(1)}.Layout(gtx, func(gtx C) D {
		return layout.Background{}.Layout(gtx, rounded(pal.Surface, 12), func(gtx C) D {
			return layout.UniformInset(unit.Dp(18)).Layout(gtx, w)
		})
	})
}

func fieldLabel(th *material.Theme, s string) layout.Widget {
	l := material.Caption(th, strings.ToUpper(s))
	l.Color = pal.Muted
	l.Font.Weight = font.SemiBold
	return l.Layout
}

func input(gtx C, th *material.Theme, ed *widget.Editor, hint string) D {
	border := pal.Border
	if gtx.Source.Focused(ed) {
		border = pal.Accent
	}
	return widget.Border{Color: border, CornerRadius: unit.Dp(8), Width: unit.Dp(1)}.Layout(gtx, func(gtx C) D {
		return layout.Background{}.Layout(gtx, rounded(pal.Surface2, 8), func(gtx C) D {
			return layout.Inset{Top: 10, Bottom: 10, Left: 12, Right: 12}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				e := material.Editor(th, ed, hint)
				e.HintColor = pal.Muted
				e.SelectionColor = withAlpha(pal.Accent, 0x60)
				return e.Layout(gtx)
			})
		})
	})
}

func button(th *material.Theme, c *widget.Clickable, label string, primary bool) layout.Widget {
	b := material.Button(th, c, label)
	b.CornerRadius = unit.Dp(8)
	// The label's line box reserves room for descenders, so button text looks
	// high when centered. Shift the padding down to center it visually.
	b.Inset = layout.Inset{Top: 13, Bottom: 7, Left: 18, Right: 18}
	b.Font.Weight = font.SemiBold
	b.TextSize = unit.Sp(14)
	if !primary {
		b.Background = pal.Surface2
		b.Color = pal.Fg
	}
	return b.Layout
}

func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

func (ui *UI) layoutSettings(gtx C, th *material.Theme) D {
	return card(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(fieldLabel(th, "Music folder")),
			vspace(6),
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						return input(gtx, th, &ui.folder, "/path/to/music")
					}),
					hspace(8),
					layout.Rigid(func(gtx C) D {
						label := "Browse…"
						if ui.browsing {
							gtx = gtx.Disabled()
							label = "Choosing…"
						}
						return button(th, &ui.browseButton, label, false)(gtx)
					}),
					hspace(8),
					layout.Rigid(button(th, &ui.scanButton, "Scan", true)),
				)
			}),
			vspace(14),
			layout.Rigid(fieldLabel(th, "Rename pattern")),
			vspace(6),
			layout.Rigid(func(gtx C) D {
				return input(gtx, th, &ui.pattern, defaultPattern)
			}),
			vspace(6),
			layout.Rigid(func(gtx C) D {
				l := material.Caption(th, "Tokens: {artist}  {title}  {bpm}  {key}   ·   zero-pad with {bpm:03}   ·   press Enter to rescan")
				l.Color = pal.Muted
				return l.Layout(gtx)
			}),
		)
	})
}

func (ui *UI) layoutResults(gtx C, th *material.Theme) D {
	gtx.Constraints.Min = gtx.Constraints.Max
	return card(gtx, func(gtx C) D {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								l := material.H6(th, "Preview")
								l.Font.Weight = font.SemiBold
								return l.Layout(gtx)
							}),
							layout.Rigid(func(gtx C) D {
								if ui.status == "" {
									return D{}
								}
								l := material.Body2(th, ui.status)
								l.Color = ui.statusKind.color()
								return l.Layout(gtx)
							}),
						)
					}),
					layout.Rigid(func(gtx C) D {
						label := "Apply changes"
						if n := len(ui.files); n > 0 {
							label = fmt.Sprintf("Rename %d files", n)
						} else {
							gtx = gtx.Disabled()
						}
						return button(th, &ui.applyButton, label, true)(gtx)
					}),
				)
			}),
			vspace(14),
			layout.Rigid(func(gtx C) D {
				if len(ui.files) == 0 {
					return D{}
				}
				return layout.Inset{Left: 10, Right: 10, Bottom: 8}.Layout(gtx, func(gtx C) D {
					return nameRow(gtx, fieldLabel(th, "Current name"), fieldLabel(th, "New name"), layout.Spacer{Width: 28}.Layout)
				})
			}),
			layout.Rigid(func(gtx C) D {
				if len(ui.files) == 0 {
					return D{}
				}
				return divider(gtx)
			}),
			layout.Flexed(1, func(gtx C) D {
				if len(ui.files) == 0 {
					return ui.layoutEmpty(gtx, th)
				}
				return material.List(th, &ui.list).Layout(gtx, len(ui.files), func(gtx C, i int) D {
					return ui.layoutRow(gtx, th, i)
				})
			}),
		)
	})
}

func (ui *UI) layoutEmpty(gtx C, th *material.Theme) D {
	msg := "Pick a folder and press Scan to preview new file names."
	if ui.scanned {
		msg = "Nothing to rename."
	}
	return layout.Center.Layout(gtx, func(gtx C) D {
		l := material.Body1(th, msg)
		l.Color = pal.Muted
		return l.Layout(gtx)
	})
}

func (ui *UI) layoutRow(gtx C, th *material.Theme, i int) D {
	f := ui.files[i]
	bg := pal.Surface
	if i%2 == 1 {
		bg = pal.Surface2
	}
	name := func(s string, c color.NRGBA) layout.Widget {
		l := material.Body2(th, s)
		l.Color = c
		l.MaxLines = 1
		return l.Layout
	}
	return layout.Background{}.Layout(gtx, rounded(bg, 6), func(gtx C) D {
		return layout.Inset{Top: 9, Bottom: 9, Left: 10, Right: 10}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			arrow := func(gtx C) D {
				return layout.Inset{Left: 8, Right: 8}.Layout(gtx, func(gtx C) D {
					l := material.Body2(th, "→")
					l.Color = pal.Accent
					return l.Layout(gtx)
				})
			}
			return nameRow(gtx, name(f.originalName, pal.Muted), name(f.newName, pal.Fg), arrow)
		})
	})
}

// nameRow lays out two equal-width columns separated by mid.
func nameRow(gtx C, left, right, mid layout.Widget) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, left),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Dp(28)
			return layout.Center.Layout(gtx, mid)
		}),
		layout.Flexed(1, right),
	)
}
