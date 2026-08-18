package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/dhowden/tag"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: deeztags <album folder>")
		return
	}

	albumDir := os.Args[1]
	fmt.Println("Fetching metadata for album: ", filepath.Base(albumDir))
	fmt.Println("Scanning folder for audio files!")

	files, err := os.ReadDir(albumDir)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if !file.IsDir() {
			fmt.Println("Found file:", file.Name())
			f, err := os.Open(filepath.Join(albumDir, file.Name()))
			if err != nil {
				log.Fatalf("Failed to open file: %v", err)
				return
			}

			m, err := tag.ReadFrom(f)
			if err != nil {
				log.Fatalf("Failed to parse tags: %v", err)
				return
			}

			fmt.Println("Format:", m.Format()) // e.g., FLAC, MP4, MP3
			fmt.Println("Title:", m.Title())
			fmt.Println("Artist:", m.Artist())
			fmt.Println("Album:", m.Album())
			fmt.Println("Year:", m.Year())
			f.Close()

		}
	}
}
