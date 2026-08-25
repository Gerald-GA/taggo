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
	"time"

	"github.com/dhowden/tag"
)

type Results struct {
	Albums []AlbumResult `json:"data"`
}

type AlbumResult struct {
	ID       int64  `json:"id"`
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

type DeezerAlbumMetadata struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Upc      string `json:"upc"`
	CoverBig string `json:"cover_big"`
	CoverXl  string `json:"cover_xl"`
	Genres   struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	} `json:"genres"`
	Label          string `json:"label"`
	TotalTracks    int    `json:"nb_tracks"`
	ReleaseDate    string `json:"release_date"`
	ExplicitLyrics bool   `json:"explicit_lyrics"`
	Artist         struct {
		Name string `json:"name"`
	} `json:"artist"`
	Tracks struct {
		Data []struct {
			ID int64 `json:"id"`
		} `json:"data"`
	} `json:"tracks"`
}

type DeezerTrackMetadata struct {
	Title          string `json:"title"`
	TrackPosition  int    `json:"track_position"`
	DiskNumber     int    `json:"disk_number"`
	ReleaseDate    string `json:"release_date"`
	ExplicitLyrics bool   `json:"explicit_lyrics"`
	Contributors   []struct {
		Name string `json:"name"`
	} `json:"contributors"`
}

type iTunesMetadata struct {
	Results []struct {
		WrapperType           string    `json:"wrapperType"`
		CollectionType        string    `json:"collectionType,omitempty"`
		CollectionID          int64     `json:"collectionId"`
		ArtistName            string    `json:"artistName"`
		CollectionName        string    `json:"collectionName"`
		ArtworkURL100         string    `json:"artworkUrl100"`
		ContentAdvisoryRating string    `json:"contentAdvisoryRating"`
		TrackCount            int       `json:"trackCount"`
		Copyright             string    `json:"copyright,omitempty"`
		ReleaseDate           time.Time `json:"releaseDate"`
		PrimaryGenreName      string    `json:"primaryGenreName"`
		Kind                  string    `json:"kind,omitempty"`
		TrackID               int64     `json:"trackId,omitempty"`
		TrackName             string    `json:"trackName,omitempty"`
		CollectionArtistName  string    `json:"collectionArtistName,omitempty"`
		TrackExplicitness     string    `json:"trackExplicitness,omitempty"`
		DiscCount             int       `json:"discCount,omitempty"`
		DiscNumber            int       `json:"discNumber,omitempty"`
		TrackNumber           int       `json:"trackNumber,omitempty"`
	} `json:"results"`
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

	var artist, album, barcode string
	// var imetadata iTunesMetadata

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

		barcode, _ = m.Raw()["barcode"].(string)
		artist = m.Artist()
		album = m.Album()
		f.Close()
	}

	if barcode != "" {
		// imetadata = iTunesLookup(barcode)
	} else {
		albums := deezerSearch(artist, album)
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
		fmt.Print("Enter the number which corresponds to the correct album: ")
		text, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal("Error reading user input")
		}
		num, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil {
			log.Fatal("Please enter a valid number")
		}
		fmt.Println(num)
		deezerLookup(albums.Albums[num-1].ID)
	}

	// for _, file := range files {
	// 	filePath := filepath.Join(albumDir, file.Name())
	// 	if !file.IsDir() && filepath.Ext(file.Name()) == ".flac" {
	// 		fmt.Println("Found file:", file.Name())
	// 		f, err := os.Open(filePath)
	// 		if err != nil {
	// 			log.Fatalf("Failed to open file: %v", err)
	// 		}

	// 		m, err := tag.ReadFrom(f)
	// 		if err != nil {
	// 			log.Fatalf("Failed to parse tags: %v", err)
	// 		}

	// 		// TODO: Sort map or create a new struct for holding metadata.
	// 		for key, value := range m.Raw() {
	// 			fmt.Printf("%s: %s\n", key, value)
	// 		}
	// 		fmt.Println()
	// 		f.Close()
	// 	}
	// }
}

func deezerSearch(artist string, album string) Results {
	var result Results
	deezerURL := fmt.Sprintf("https://api.deezer.com/search/album/?q=%q&limit=10", url.QueryEscape(artist+" "+album))
	resp, err := http.Get(deezerURL)
	if err != nil {
		log.Fatal("HTTP GET request failed, ", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Failed to read resp.Body, ", err)
	}
	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Fatal("Error while processing JSON response:", err)
	}
	return result
}

func deezerLookup(id int64) {
	var albumMetadata DeezerAlbumMetadata
	// var trackMetadata []DeezerTrackMetadata
	var trackIDs []int64

	deezerURL := fmt.Sprintf("https://api.deezer.com/album/%d", id)
	resp, err := http.Get(deezerURL)
	if err != nil {
		log.Fatal("HTTP GET request failed, ", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Failed to read resp.Body, ", err)
	}
	err = json.Unmarshal(body, &albumMetadata)
	if err != nil {
		log.Fatal("Error while processing JSON response:", err)
	}
	for _, track := range albumMetadata.Tracks.Data {
		trackIDs = append(trackIDs, track.ID)
	}
	if len(trackIDs) != albumMetadata.TotalTracks {
		log.Fatalf("Total track IDs (%d) does not match album's reported track count (%d)", len(trackIDs), albumMetadata.TotalTracks)
	} else {
		fmt.Printf("Total track IDs (%d) matches album's reported track count (%d)\n", len(trackIDs), albumMetadata.TotalTracks)
	}
}

func iTunesLookup(barcode string) {
	// TODO
}
