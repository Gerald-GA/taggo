package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/dhowden/tag"
)

type Parameters struct {
	Artist  string
	Album   string
	Barcode string
}

type Metadata struct {
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: taggo <album folder>")
		return
	}

	albumDir := os.Args[1]
	fmt.Println("Fetching metadata for album: ", filepath.Base(albumDir))
	fmt.Println("Scanning folder for audio files!")

	files, err := os.ReadDir(albumDir)
	if err != nil {
		log.Fatal(err)
	}

	params := Parameters{
		Artist:  "Kendo Kaponi",
		Album:   "APOCALIPTO",
		Barcode: "",
	}
	getMetadata(params)

	for _, file := range files {
		filePath := filepath.Join(albumDir, file.Name())
		if !file.IsDir() && filepath.Ext(file.Name()) == ".flac" {
			fmt.Println("Found file:", file.Name())
			f, err := os.Open(filePath)
			if err != nil {
				log.Fatalf("Failed to open file: %v", err)
			}

			m, err := tag.ReadFrom(f)
			if err != nil {
				log.Fatalf("Failed to parse tags: %v", err)
			}

			// TODO: Sort map or create a new struct for holding metadata.
			for key, value := range m.Raw() {
				fmt.Printf("%s: %s\n", key, value)
			}
			fmt.Println()
			f.Close()
		}
	}
}

func getMetadata(params Parameters) {
	deezerURL := "https://api.deezer.com/"

	if params.Barcode != "" {
		deezerURL += "album/upc:" + params.Barcode
	} else {
		deezerURL += fmt.Sprintf("search/?q=artist:%qalbum:%q", url.QueryEscape(params.Artist), url.QueryEscape(params.Album))
	}

	fmt.Println(deezerURL)

	resp, err := http.Get(deezerURL)
	if err != nil {
		log.Fatal("HTTP GET request failedd")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Failed to read resp.Body")
	}
	fmt.Println(string(body))
}
