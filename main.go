package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bogem/id3v2/v2"
)

func main() {
	argLength := len(os.Args[1:])
	fmt.Printf("Arg length is %d\n", argLength)
	if argLength == 0 {
		fmt.Println("No arguments provided")
		return
	}
	path := os.Args[1]
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		fmt.Println("Path given does not exist")
	}
	if !info.IsDir() {
		fmt.Println("Path given is not a valid folder")
	}

	dirEntries, err := os.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}

	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() {
			continue
		}
		if filepath.Ext(dirEntry.Name()) != ".mp3" {
			continue
		}
		fileName := dirEntry.Name()
		fpath := filepath.Join(path, fileName)
		m, err := id3v2.Open(fpath, id3v2.Options{Parse: true})
		if err != nil {
			log.Fatal("Error while opening mp3 file: ", err)
		}
		defer m.Close()

		//fmt.Println(fpath)
		//fmt.Println(m.Artist(), ";", m.Title(), ";", m.Album())
		//split := strings.Split(m.Title(), " - ")

		bpm := m.GetTextFrame("TBPM").Text
		key := m.GetTextFrame("TKEY").Text
		song := m.GetTextFrame("TIT2").Text
		if strings.Index(song, " - ") < 0 {
			song = fmt.Sprintf("%s - %s", m.Artist(), m.Title())
		} else if strings.Index(song, " - ") > 0 {
			split := strings.Split(song, " - ")
			artist := split[0]
			title := split[1]
			if artist != m.Artist() {
				m.SetArtist(artist)
			}
			if title != m.Title() {
				m.SetTitle(title)
			}
			m.Save()
		}
		//artist := split[0]
		//title := split[1]
		newName := fmt.Sprintf("%03s %03s %s.mp3", bpm, key, song)
		newPath := filepath.Join(path, newName)
		if newName == fileName {
			continue
		}
		fmt.Println(newPath)
		err = os.Rename(fpath, newPath)
		if err != nil {
			log.Fatal(err)
		}

	}
}
