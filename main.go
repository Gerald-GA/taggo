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

	"go.senan.xyz/taglib"
)

type iTunesResults struct {
	ResultCount int `json:"resultCount"`
	Results     []struct {
		ArtistName             string    `json:"artistName"`
		CollectionName         string    `json:"collectionName"`
		CollectionExplicitness string    `json:"collectionExplicitness"`
		CollectionViewURL      string    `json:"collectionViewUrl"`
		CollectionID           int64     `json:"collectionId"`
		TrackCount             int       `json:"trackCount"`
		ReleaseDate            time.Time `json:"releaseDate"`
	} `json:"results"`
}

type DeezerResults struct {
	Albums []struct {
		Title  string `json:"title"`
		Upc    string `json:"upc"`
		Link   string `json:"tracklist"`
		Artist struct {
			Name string `json:"name"`
			ID   int    `json:"id"`
		} `json:"artist"`
		ID       int64 `json:"id"`
		NbTracks int   `json:"nb_tracks"`
		Explicit bool  `json:"explicit_lyrics"`
	} `json:"data"`
}

type DeezerAlbumJSON struct {
	Title       string `json:"title"`
	Upc         string `json:"upc"`
	CoverBig    string `json:"cover_big"`
	Label       string `json:"label"`
	ReleaseDate string `json:"release_date"`
	Artist      struct {
		Name string `json:"name"`
	} `json:"artist"`
	Genres struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	} `json:"genres"`
	Tracks struct {
		Data []struct {
			ID int64 `json:"id"`
		} `json:"data"`
	} `json:"tracks"`
	ID             int64 `json:"id"`
	TotalTracks    int   `json:"nb_tracks"`
	ExplicitLyrics bool  `json:"explicit_lyrics"`
}

type DeezerTrackJSON struct {
	Title        string `json:"title"`
	ReleaseDate  string `json:"release_date"`
	Contributors []struct {
		Name string `json:"name"`
	} `json:"contributors"`
	TrackPosition  int  `json:"track_position"`
	DiskNumber     int  `json:"disk_number"`
	ExplicitLyrics bool `json:"explicit_lyrics"`
}

type iTunesJSON struct {
	Results []struct {
		ReleaseDate            time.Time `json:"releaseDate"`
		PrimaryGenreName       string    `json:"primaryGenreName"`
		TrackName              string    `json:"trackName,omitempty"`
		ArtworkURL100          string    `json:"artworkUrl100"`
		CollectionExplicitness string    `json:"collectionExplicitness"`
		ArtistName             string    `json:"artistName"`
		Copyright              string    `json:"copyright,omitempty"`
		CollectionName         string    `json:"collectionName"`
		TrackExplicitness      string    `json:"trackExplicitness,omitempty"`
		Kind                   string    `json:"kind,omitempty"`
		TrackCount             int       `json:"trackCount"`
		CollectionID           int64     `json:"collectionId"`
		TrackID                int64     `json:"trackId,omitempty"`
		DiscCount              int       `json:"discCount,omitempty"`
		DiscNumber             int       `json:"discNumber,omitempty"`
		TrackNumber            int       `json:"trackNumber,omitempty"`
	} `json:"results"`
}

type TrackMetadata struct {
	Copyright     string
	AlbumUPC      string
	TrackArtist   string
	AlbumName     string
	AlbumArtist   string
	TrackTitle    string
	ReleaseDate   string
	Genre         string
	Kind          string
	CoverURL      string
	CollectionID  string
	TrackID       string
	DeezerTrackID string
	DeezerAlbumID string
	TotalDiscs    int
	TotalTracks   int
	TrackNumber   int
	TrackDisc     int
	AlbumExplict  bool
	TrackExplicit bool
}

type AudioFile struct {
	Path        string
	Name        string
	TrackNumber int
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: taggo <album folder>")
		return
	}

	albumDir := os.Args[1]

	files, err := os.ReadDir(albumDir)
	if err != nil {
		log.Fatalf("Crash: %s", err)
	}

	var artist, album, barcode string
	var metadata []TrackMetadata
	var filelist []AudioFile

	for _, file := range files {
		if !file.IsDir() && strings.EqualFold(filepath.Ext(file.Name()), ".flac") {
			filePath := filepath.Join(albumDir, file.Name())
			tags, err := taglib.ReadTags(filePath)
			if err != nil {
				log.Fatalf("Failed to parse tags: %v", err)
			}

			// TODO: Add backup for tagless tracks, eg "track 01.flac"
			if len(tags[taglib.TrackNumber]) == 0 {
				log.Printf("Skipping %s: missing track number tag", file.Name())
				continue
			}

			var tracknum int
			if strings.Contains(tags[taglib.TrackNumber][0], "/") {
				tracknum, err = strconv.Atoi(strings.Split(tags[taglib.TrackNumber][0], "/")[0])
			} else {
				tracknum, err = strconv.Atoi(tags[taglib.TrackNumber][0])
			}

			if err != nil {
				log.Printf("Skipping %s: Error converting track number (%s) to int", file.Name(), tags[taglib.TrackNumber][0])
				continue
			}

			filelist = append(filelist, AudioFile{Path: filePath, Name: file.Name(), TrackNumber: tracknum})

			if artist == "" || album == "" {
				getFirst := func(field string) string {
					if vals, ok := tags[field]; ok && len(vals) > 0 {
						return vals[0]
					}
					return ""
				}

				barcode = getFirst(taglib.Barcode)
				artist = getFirst(taglib.AlbumArtist)
				album = getFirst(taglib.Album)
			}
		}
	}

	fmt.Printf("Album loaded: %s - %s (%d Tracks)\n\n", artist, album, len(files))

	if barcode == "" {
		if artist == "" || album == "" {
			log.Fatal("Could not find artist or album tags in folder files, and no barcode present.")
		}
		fmt.Printf("No Barcode found, searching Deezer...\n\n")
		albums := deezerSearch(artist, album)
		if len(albums.Albums) == 0 {
			log.Fatal("No matching albums found! Exiting...")
		}
		for i, a := range albums.Albums {
			advisory := "explicit"
			if !a.Explicit {
				advisory = "clean"
			}
			fmt.Printf("%d: %s - %s (%d Tracks | %s)\n", i+1, a.Artist.Name, a.Title, a.NbTracks, advisory)
		}
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("\nEnter the number which corresponds to the correct album: ")
		text, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal("Error reading user input")
		}
		num, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || num < 1 || num > len(albums.Albums) {
			log.Fatal("Please enter a valid selection")
		}
		barcode = deezerUPCLookup(albums.Albums[num-1].ID)
	}

	fmt.Println("looking up Barcode in iTunes...")
	results := iTunesSearch(barcode)

	switch results.ResultCount {
	case 0:
		log.Fatal("Unable to find album")
	case 1:
		metadata = iTunesLookup(barcode, results.Results[0].CollectionID)
	default:
		for i, album := range results.Results {
			fmt.Printf("%d: %s - %s (%d) (%d Tracks | %s) Link: %s\n", i+1, album.ArtistName, album.CollectionName, album.ReleaseDate.Year(), album.TrackCount, album.CollectionExplicitness, album.CollectionViewURL)
		}
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("\nEnter the number which corresponds to the correct album: ")
		text, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal("Error reading user input")
		}
		num, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || num < 1 || num > results.ResultCount {
			log.Fatal("Please enter a valid selection")
		}
		metadata = iTunesLookup(barcode, results.Results[num-1].CollectionID)
	}

	// TODO: Add deezer lookup fallback
	if len(metadata) == 0 {
		log.Fatal("API returned empty metadata list!")
	}

	fmt.Println("Please confirm that the following is correct")
	Confirm(filelist, metadata)

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nDoes this look correct? (y/n): \n")
	text, _ := reader.ReadString('\n')
	fmt.Println()
	if strings.ToLower(strings.TrimSpace(text)) != "y" {
		fmt.Println("Aborted by user.")
		return
	}

	// Download cover art once
	cover, err := fetchURL(metadata[0].CoverURL)
	if err != nil {
		fmt.Println("Faild to download cover art:", err)
	} else {
		fmt.Println("Successfully downloaded cover art")
	}

	for _, file := range filelist {
		// Prevent index out of range if metadata slice is smaller than tracknum
		idx := file.TrackNumber - 1
		if idx < 0 || idx >= len(metadata) {
			log.Printf("Skipping %s: track number %d exceeds API metadata length (%d)", file.Name, file.TrackNumber, len(metadata))
			continue
		}

		fmt.Printf("Processing Track %d: %s\n", file.TrackNumber, file.Name)

		// Write Text Tags
		err = taglib.WriteTags(file.Path, metadata[idx].ToTagMap(), taglib.Clear)
		if err != nil {
			log.Printf("Failed to write tags for %s: %v", file.Name, err)
			continue
		}

		// Write Cover Art (if downloaded successfully)
		if len(cover) > 0 {
			err = taglib.WriteImage(file.Path, cover)
			if err != nil {
				fmt.Printf("Failed to write cover art to %s: %v\n", file.Name, err)
			}
		}
	}
	fmt.Println("Done tagging album!")
}

func deezerSearch(artist string, album string) DeezerResults {
	var result DeezerResults
	body, err := fetchURL(fmt.Sprintf("https://api.deezer.com/search/album/?q=%q&limit=10", url.QueryEscape(artist+" "+album)))
	if err != nil {
		log.Fatal("Failed to search album in Deezer database:", err)
	}
	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Fatal("Error while processing JSON response:", err)
	}
	return result
}

func deezerUPCLookup(id int64) string {
	var albumMetadata DeezerAlbumJSON
	body, err := fetchURL(fmt.Sprintf("https://api.deezer.com/album/%d", id))
	if err != nil {
		log.Fatal("Failed to lookup album metadata in Deezer database:", err)
	}
	err = json.Unmarshal(body, &albumMetadata)
	if err != nil {
		log.Fatal("Error while processing JSON response:", err)
	}
	return albumMetadata.Upc
}

func deezerLookup(id int64) []TrackMetadata {
	var albumMetadata DeezerAlbumJSON
	var trackMetadata []DeezerTrackJSON
	var trackIDs []int64

	body, err := fetchURL(fmt.Sprintf("https://api.deezer.com/album/%d", id))
	if err != nil {
		log.Fatal("Failed to lookup album metadata in Deezer database:", err)
	}
	err = json.Unmarshal(body, &albumMetadata)
	if err != nil {
		log.Fatal("Error while processing JSON response:", err)
	}
	for _, trackJSON := range albumMetadata.Tracks.Data {
		trackIDs = append(trackIDs, trackJSON.ID)
	}
	if len(trackIDs) != albumMetadata.TotalTracks {
		log.Fatalf("Total track IDs (%d) does not match album's reported track count (%d)", len(trackIDs), albumMetadata.TotalTracks)
	}

	for _, trackID := range trackIDs {
		body, err := fetchURL(fmt.Sprintf("https://api.deezer.com/track/%d", trackID))
		if err != nil {
			log.Fatal("Failed to lookup track metadata in Deezer database:", err)
		}
		var tmp DeezerTrackJSON
		err = json.Unmarshal(body, &tmp)
		if err != nil {
			log.Fatal("Unable to parse JSON")
		}
		trackMetadata = append(trackMetadata, tmp)
	}
	return deezerDecode(albumMetadata, trackMetadata)
}

func iTunesSearch(UPC string) iTunesResults {
	var result iTunesResults
	body, err := fetchURL(fmt.Sprintf("https://itunes.apple.com/lookup?upc=%s&entity=album", UPC))
	if err != nil {
		log.Fatal("Error looking up album in iTunes database:", err)
	}
	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Fatal("Error while processing JSON response:", err)
	}
	return result
}

func iTunesLookup(UPC string, ID int64) []TrackMetadata {
	var result iTunesJSON
	body, err := fetchURL(fmt.Sprintf("https://itunes.apple.com/lookup?id=%d&entity=song", ID))
	if err != nil {
		log.Fatal("Error looking up album in iTunes database:", err)
	}
	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Fatal("Error while processing JSON response:", err)
	}
	return itunesDecode(result, UPC)
}

func fetchURL(URL string) ([]byte, error) {
	resp, err := http.Get(URL)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP Error code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read resp.Body: %w", err)
	}

	return body, nil
}

func itunesDecode(album iTunesJSON, UPC string) []TrackMetadata {
	if len(album.Results) == 0 {
		log.Fatal("Didn't get any results for album")
	}

	if len(album.Results) != album.Results[0].TrackCount+1 {
		log.Fatal("Number of returned tracks from iTunes does not match reported Track Count")
	}

	var metadata []TrackMetadata
	releaseDate := album.Results[0].ReleaseDate.Format("2006-01-02")
	highResCover := strings.Replace(album.Results[0].ArtworkURL100, "100x100", "800x800", 1)

	// Starts at index 1 because the first result from iTunes is the album collection, after that follows each track
	for i := 1; i <= album.Results[0].TrackCount; i++ {
		var tmp TrackMetadata

		if album.Results[0].CollectionExplicitness == "explicit" {
			tmp.AlbumExplict = true
		} else {
			tmp.AlbumExplict = false
		}

		if album.Results[i].TrackExplicitness == "explicit" {
			tmp.TrackExplicit = true
		} else {
			tmp.TrackExplicit = false
		}
		tmp.AlbumUPC = UPC
		tmp.CollectionID = strconv.FormatInt(album.Results[0].CollectionID, 10)
		tmp.AlbumName = album.Results[0].CollectionName
		tmp.AlbumArtist = album.Results[0].ArtistName
		tmp.TotalTracks = album.Results[0].TrackCount
		tmp.TotalDiscs = album.Results[0].DiscCount
		tmp.Genre = album.Results[0].PrimaryGenreName
		tmp.ReleaseDate = releaseDate
		tmp.Copyright = album.Results[0].Copyright
		tmp.CoverURL = highResCover
		tmp.Kind = album.Results[i].Kind
		tmp.TrackID = strconv.FormatInt(album.Results[i].TrackID, 10)
		tmp.TrackTitle = album.Results[i].TrackName
		tmp.TrackArtist = album.Results[i].ArtistName
		tmp.TrackNumber = album.Results[i].TrackNumber
		tmp.TrackDisc = album.Results[i].DiscNumber
		metadata = append(metadata, tmp)
		// Debugging
		prettyStruct, _ := json.MarshalIndent(tmp, "", "  ")
		fmt.Println(string(prettyStruct))
	}
	return metadata
}

func deezerDecode(album DeezerAlbumJSON, tracks []DeezerTrackJSON) []TrackMetadata {
	var metadata []TrackMetadata
	highResCover := strings.Replace(album.CoverBig, "500x500", "800x800", 1)
	totalDiscs := 1

	for _, track := range tracks {
		if track.DiskNumber > totalDiscs {
			totalDiscs = track.DiskNumber
		}
	}

	genre := "Unknown"
	if len(album.Genres.Data) > 0 {
		genre = album.Genres.Data[0].Name
	}

	for i, track := range tracks {
		var tmp TrackMetadata
		tmp.AlbumUPC = album.Upc
		tmp.DeezerAlbumID = strconv.FormatInt(album.ID, 10)
		tmp.DeezerTrackID = strconv.FormatInt(album.Tracks.Data[i].ID, 10)
		tmp.AlbumExplict = album.ExplicitLyrics
		tmp.AlbumName = album.Title
		tmp.AlbumArtist = album.Artist.Name
		tmp.TotalTracks = album.TotalTracks
		tmp.TotalDiscs = totalDiscs
		tmp.Genre = genre
		tmp.ReleaseDate = album.ReleaseDate
		tmp.Copyright = album.Label
		tmp.CoverURL = highResCover
		tmp.TrackExplicit = track.ExplicitLyrics
		tmp.TrackTitle = track.Title

		var artists []string
		for _, artist := range track.Contributors {
			artists = append(artists, artist.Name)
		}

		tmp.TrackArtist = strings.Join(artists, ", ")
		tmp.TrackNumber = track.TrackPosition
		tmp.TrackDisc = track.DiskNumber
		metadata = append(metadata, tmp)
	}
	return metadata
}

func (t TrackMetadata) ToTagMap() map[string][]string {
	tags := make(map[string][]string)

	if t.AlbumName != "" {
		tags[taglib.Album] = []string{t.AlbumName}
	}
	if t.AlbumArtist != "" {
		tags[taglib.AlbumArtist] = []string{t.AlbumArtist}
	}
	if t.TrackTitle != "" {
		tags[taglib.Title] = []string{t.TrackTitle}
	}
	if t.TrackArtist != "" {
		tags[taglib.Artist] = []string{t.TrackArtist}
	}
	if t.Genre != "" {
		tags[taglib.Genre] = []string{t.Genre}
	}
	if t.ReleaseDate != "" {
		tags[taglib.Date] = []string{t.ReleaseDate}
	}
	if t.Copyright != "" {
		tags[taglib.Copyright] = []string{t.Copyright}
	}
	if t.TrackNumber > 0 {
		tags[taglib.TrackNumber] = []string{strconv.Itoa(t.TrackNumber)}
	}
	if t.TotalTracks > 0 {
		tags["TRACKTOTAL"] = []string{strconv.Itoa(t.TotalTracks)}
	}
	if t.TrackDisc > 0 {
		tags[taglib.DiscNumber] = []string{strconv.Itoa(t.TrackDisc)}
	}
	if t.AlbumUPC != "" {
		tags[taglib.Barcode] = []string{t.AlbumUPC}
	}
	if t.CollectionID != "" {
		tags["ITUNESALBUMID"] = []string{t.CollectionID}
	}
	if t.TrackID != "" {
		tags["ITUNESTRACKID"] = []string{t.TrackID}
	}
	if t.DeezerAlbumID != "" {
		tags["DEEZERALBUMID"] = []string{t.DeezerAlbumID}
	}
	if t.TrackID != "" {
		tags["DEEZERTRACKID"] = []string{t.DeezerTrackID}
	}

	return tags
}

func Confirm(filelist []AudioFile, metadata []TrackMetadata) {
	type printJob struct {
		oldName string
		newName string
	}
	var jobs []printJob
	var longest int

	// Phase 1: Collect names and find the longest filename
	for _, file := range filelist {
		s1 := file.Name
		s2 := fmt.Sprintf("%d - %s", metadata[file.TrackNumber-1].TrackNumber, metadata[file.TrackNumber-1].TrackTitle)

		if len(s1) > longest {
			longest = len(s1)
		}

		jobs = append(jobs, printJob{oldName: s1, newName: s2})
	}

	// Phase 2: Print with dynamic arrow padding
	for _, job := range jobs {
		// Calculate how many extra dashes we need so the arrows align perfectly
		padLength := longest - len(job.oldName)

		// Create the arrow: space + (base dashes + extra padding dashes) + "> "
		// If it's the longest filename, padLength is 0, so it gets exactly 3 dashes
		arrow := " " + strings.Repeat("-", padLength+3) + "> "

		fmt.Println(job.oldName + arrow + job.newName)
	}
}
