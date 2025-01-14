package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"gioui.org/app"
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
	selectedFolder string
	pattern        widget.Editor
	selectButton   widget.Clickable
	applyButton    widget.Clickable
	files          []FileRename
	list           layout.List
}

func main() {
	go func() {
		w := new(app.Window)
		if err := loop(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func loop(w *app.Window) error {
	th := material.NewTheme()
	var ops op.Ops
	ui := &UI{
		pattern: widget.Editor{SingleLine: true},
		list:    layout.List{Axis: layout.Vertical},
	}
	ui.pattern.SetText("{bpm} {key} {artist} - {title}")
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)

			if ui.selectButton.Clicked(gtx) {
				// TODO: Implement folder selection dialog
				ui.selectedFolder = "/path/to/folder"
				ui.scanFiles()
			}

			if ui.applyButton.Clicked(gtx) && len(ui.files) > 0 {
				ui.applyRenames()
			}

			layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(material.Button(th, &ui.selectButton, "Select Folder").Layout),
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
			e.Frame(gtx.Ops)
		}
	}
}

func (ui *UI) scanFiles() {
	ui.files = nil
	dirEntries, err := os.ReadDir(ui.selectedFolder)
	if err != nil {
		return
	}

	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() || filepath.Ext(dirEntry.Name()) != ".mp3" {
			continue
		}
		fileName := dirEntry.Name()
		fpath := filepath.Join(ui.selectedFolder, fileName)
		m, err := id3v2.Open(fpath, id3v2.Options{Parse: true})
		if err != nil {
			continue
		}
		defer m.Close()

		newName := ui.generateNewName(m)
		ui.files = append(ui.files, FileRename{
			originalName: fileName,
			newName:      newName,
		})
	}
}

func (ui *UI) generateNewName(m *id3v2.Tag) string {
	pattern := ui.pattern.Text()
	name := pattern
	name = strings.ReplaceAll(name, "{bpm}", m.GetTextFrame("TBPM").Text)
	name = strings.ReplaceAll(name, "{key}", m.GetTextFrame("TKEY").Text)
	name = strings.ReplaceAll(name, "{artist}", m.Artist())
	name = strings.ReplaceAll(name, "{title}", m.Title())
	return name + ".mp3"
}

func (ui *UI) applyRenames() {
	for _, file := range ui.files {
		oldPath := filepath.Join(ui.selectedFolder, file.originalName)
		newPath := filepath.Join(ui.selectedFolder, file.newName)
		if oldPath != newPath {
			os.Rename(oldPath, newPath)
		}
	}
	ui.scanFiles() // Refresh the list
}
