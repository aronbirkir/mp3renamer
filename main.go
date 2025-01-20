package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bogem/id3v2/v2"
)

type FileRename struct {
	originalName string
	newName      string
}

type UI struct {
	folder      widget.Editor
	pattern     widget.Editor
	scanButton  widget.Clickable
	applyButton widget.Clickable
	files       []FileRename
	list        layout.List
}

func main() {
	go func() {
		window := new(app.Window)
		err := run(window)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(window *app.Window) error {
	theme := material.NewTheme()
	var ops op.Ops
	ui := &UI{
		folder:  widget.Editor{SingleLine: true},
		pattern: widget.Editor{SingleLine: true},
		list:    layout.List{Axis: layout.Vertical},
	}
	ui.pattern.SetText("{bpm:03} {key:03} {artist} - {title}")
	for {
		switch e := window.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)

			if ui.scanButton.Clicked(gtx) {
				//ui.folder = "/path/to/folder"
				ui.scanFiles()
			}

			if ui.applyButton.Clicked(gtx) && len(ui.files) > 0 {
				ui.applyRenames()
			}
			ui.Layout(gtx, theme)
			e.Frame(gtx.Ops)
		}
	}
}

func (ui *UI) scanFiles() {
	ui.files = nil
	path := ui.folder.Text()
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return
	}

	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() || filepath.Ext(dirEntry.Name()) != ".mp3" {
			continue
		}
		fileName := dirEntry.Name()
		filePath := filepath.Join(path, fileName)
		m, err := id3v2.Open(filePath, id3v2.Options{Parse: true})
		if err != nil {
			continue
		}
		defer m.Close()

		newName := ui.generateNewName(m)
		if newName != fileName {
			ui.files = append(ui.files, FileRename{
				originalName: fileName,
				newName:      newName,
			})
		}
	}
}

func (ui *UI) generateNewName(m *id3v2.Tag) string {
	pattern := ui.pattern.Text()
	songInfo := map[string]string{
		"artist": m.Artist(),
		"title":  m.Title(),
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
	for _, file := range ui.files {
		oldPath := filepath.Join(path, file.originalName)
		newPath := filepath.Join(path, file.newName)
		if oldPath != newPath {
			os.Rename(oldPath, newPath)
		}
	}
	ui.scanFiles() // Refresh the list
}

func (ui *UI) Layout(gtx C, th *material.Theme) D {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return Rect{
				Color: backgroundColor,
				Size: f32.Point{
					X: float32(gtx.Constraints.Max.X),
					Y: float32(gtx.Constraints.Max.Y),
				},
			}.Layout(gtx)
		}),
		layout.Stacked(func(gtx C) D {
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return Rect{
						Color: backgroundColor,
						Size: f32.Point{
							X: float32(gtx.Constraints.Max.X),
							Y: float32(gtx.Constraints.Max.Y),
						},
					}.Layout(gtx)
				}),
				layout.Stacked(func(gtx C) D {
					return ui.LayoutForm(gtx, th)
				}),
			)
		}),
	)
}

func (ui *UI) LayoutForm(gtx C, th *material.Theme) D {
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D {
					return layout.UniformInset(unit.Dp(4)).Layout(gtx,
						material.Body1(th, "Music folder:").Layout,
					)
				})
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D {
					return layout.UniformInset(unit.Dp(4)).Layout(gtx,
						material.Editor(th, &ui.folder, "Enter Folder Path").Layout)
				})
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D {
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.UniformInset(unit.Dp(4)).Layout(gtx,
								material.Button(th, &ui.scanButton, "Scan").Layout)
						}),
						layout.Rigid(func(gtx C) D {
							if len(ui.files) > 0 {
								return layout.UniformInset(unit.Dp(4)).Layout(gtx,
									material.Button(th, &ui.applyButton, "Apply Changes").Layout)
							}
							return layout.Dimensions{}
						}),
					)
				})
			}),
			layout.Rigid(func(gtx C) D {
				if ui.folder.Text() != "" {
					return layout.Center.Layout(gtx, func(gtx C) D {
						return layout.UniformInset(unit.Dp(4)).Layout(gtx,
							material.Body2(th, "Music files:").Layout,
						)
					})
				}
				return layout.Dimensions{}
			}),
			layout.Rigid(func(gtx C) D {
				if ui.folder.Text() != "" {
					return ui.list.Layout(gtx, len(ui.files), func(gtx C, i int) D {
						return layout.Flex{}.Layout(gtx,
							layout.Flexed(1, material.Label(th, unit.Sp(14), ui.files[i].originalName).Layout),
							layout.Flexed(1, material.Label(th, unit.Sp(14), ui.files[i].newName).Layout),
						)
					})
				}
				return layout.Dimensions{}
			}),
		)
	})
}

func (ui *UI) Layout_old(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(1, material.Editor(th, &ui.folder, "Folder").Layout),
				layout.Rigid(material.Button(th, &ui.scanButton, "Scan").Layout),
				layout.Flexed(1, material.Editor(th, &ui.pattern, "Pattern").Layout),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.list.Layout(gtx, len(ui.files), func(gtx layout.Context, i int) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(1, material.Label(th, unit.Sp(14), ui.files[i].originalName).Layout),
					layout.Flexed(1, material.Label(th, unit.Sp(14), ui.files[i].newName).Layout),
				)
			})
		}),
		layout.Rigid(material.Button(th, &ui.applyButton, "Apply Changes").Layout),
	)
}
