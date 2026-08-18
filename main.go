package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: deeztags <album folder>")
		return
	}

	album := os.Args[1]
	fmt.Println("Fetching metadata for album: ", filepath.Base(album))
	fmt.Println("Scanning folder for audio files!")

	files, err := os.ReadDir(album)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if !file.IsDir() {
			fmt.Println("Found file:", file.Name())
		}
	}
}
