package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dhowden/tag"
)

type Parameters struct {
	Artist  string
	Album   string
	Barcode string
}

type AlbumResults struct {
	Albums []Album `json:"data"`
}

type Album struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Upc      string `json:"upc"`
	Link     string `json:"tracklist"`
	NbTracks int    `json:"nb_tracks"`
	Explicit bool   `json:"explicit_lyrics"`
	Artist   struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"artist"`
}

type Metadata struct {
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: taggo <album folder>")
		return
	}

	albumDir := os.Args[1]
	fmt.Println("Folder loaded: ", filepath.Base(albumDir))
	fmt.Println("Scanning first track for existing metadata")

	files, err := os.ReadDir(albumDir)
	if err != nil {
		log.Fatal(err)
	}

	var params Parameters

	filePath := filepath.Join(albumDir, files[0].Name())
	if !files[0].IsDir() && filepath.Ext(files[0].Name()) == ".flac" {
		f, err := os.Open(filePath)
		if err != nil {
			log.Fatalf("Failed to open file: %v", err)
		}

		m, err := tag.ReadFrom(f)
		if err != nil {
			log.Fatalf("Failed to parse tags: %v", err)
		}

		params.Artist = m.Artist()
		params.Album = m.Album()

		barcodeString, ok := m.Raw()["barcode"].(string)
		if ok {
			params.Barcode = barcodeString
		}

		f.Close()
	}

	albums := search(params)
	if len(albums.Albums) == 0 {
		log.Fatal("No matching albums found! Exiting...")
	}

	for i, album := range albums.Albums {
		advisory := "explicit"
		if !album.Explicit {
			advisory = "clean"
		}
		fmt.Printf("%d: %s - %s (%d Tracks | %s)\n", i+1, album.Title, album.Artist.Name, album.NbTracks, advisory)
	}
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter the number which coresponds to the correct album: ")
	text, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal("Error reading user input")
	}
	num, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil {
		log.Fatal("Please enter a valid number")
	}
	fmt.Println(num)

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

func search(params Parameters) AlbumResults {
	deezerURL := "https://api.deezer.com/"
	var result AlbumResults

	if params.Barcode != "" {
		var response Album
		deezerURL += "album/upc:" + params.Barcode
		err := json.Unmarshal(lookup(deezerURL), &response)
		if err != nil {
			log.Fatal("Error while processing JSON response:", err)
		}
		result.Albums = append(result.Albums, response)
	} else {
		deezerURL += fmt.Sprintf("search/album/?q=%q&limit=10", url.QueryEscape(params.Artist+" "+params.Album))
		err := json.Unmarshal(lookup(deezerURL), &result)
		if err != nil {
			log.Fatal("Error while processing JSON response:", err)
		}
	}
	return result
}

func lookup(deezerURL string) []byte {
	resp, err := http.Get(deezerURL)
	if err != nil {
		log.Fatal("HTTP GET request failed, ", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Failed to read resp.Body, ", err)
	}
	return body
}
