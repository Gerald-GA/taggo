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
		ReleaseDate       time.Time `json:"releaseDate"`
		PrimaryGenreName  string    `json:"primaryGenreName"`
		TrackName         string    `json:"trackName,omitempty"`
		ArtworkURL100     string    `json:"artworkUrl100"`
		ArtistName        string    `json:"artistName"`
		Copyright         string    `json:"copyright,omitempty"`
		CollectionName    string    `json:"collectionName"`
		TrackExplicitness string    `json:"trackExplicitness,omitempty"`
		Kind              string    `json:"kind,omitempty"`
		TrackCount        int       `json:"trackCount"`
		CollectionID      int64     `json:"collectionId"`
		TrackID           int64     `json:"trackId,omitempty"`
		DiscCount         int       `json:"discCount,omitempty"`
		DiscNumber        int       `json:"discNumber,omitempty"`
		TrackNumber       int       `json:"trackNumber,omitempty"`
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
	TrackExplicit bool
}

type AudioFile struct {
	Path        string
	Name        string
	TrackNumber int
}

type SearchQuery struct {
	Artist           string
	Album            string
	UPC              string
	ItunesID         string
	DeezerID         string
	DeezerIDFallback int64
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

	var query SearchQuery
	var metadata []TrackMetadata
	var filelist []AudioFile
	var flac bool

	for _, file := range files {
		if !file.IsDir() && strings.EqualFold(filepath.Ext(file.Name()), ".flac") {
			flac = true
			filePath := filepath.Join(albumDir, file.Name())
			tags, err := taglib.ReadTags(filePath)
			if err != nil {
				log.Fatalf("Failed to parse tags: %v", err)
			}

			// TODO: Add backup method for tagless tracks, eg "track 01.flac"
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

			if query.Artist == "" || query.Album == "" {
				getFirst := func(field string) string {
					if vals, ok := tags[field]; ok && len(vals) > 0 {
						return vals[0]
					}
					return ""
				}

				query.UPC = getFirst(taglib.Barcode)
				query.Artist = getFirst(taglib.AlbumArtist)
				query.Album = getFirst(taglib.Album)
			}
		}
	}

	if !flac {
		fmt.Println("No FLAC/Compatible files found")
		return
	}

	// If the folder didn't have existing tags to scan, force manual mode immediately
	if query.UPC == "" && query.Artist == "" && query.Album == "" {
		fmt.Println("No existing tags found in files.")
		query = getManualQuery()
	} else {
		fmt.Printf("Album loaded: %s - %s (%d Tracks)\n\n", query.Artist, query.Album, len(files))
	}

	// Tries to do an automatic metadata query with scanned tags.
	metadata = resolveMetadata(query)

	// If unable to get metadata from automatic search, fallback to manual
	if len(metadata) == 0 {
		fmt.Println("\nUnable to automatically retrieve metadata, switching to manual search")
		metadata = resolveMetadata(getManualQuery())
	}

	// If manual search fails to find matches, exit.
	if len(metadata) == 0 {
		log.Fatal("Unable to find matching metadata")
	}

	// Asks the user if the metadata found matches the files to be tagged
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
	cover, err := fetchCover(metadata[0].CoverURL)
	if err != nil {
		fmt.Println("Failed to download cover art:", err)
	}

	// Loops through each track and tags
	for _, file := range filelist {
		// Prevent index out of range if returned album metadata has less tracks than folder
		idx := file.TrackNumber - 1
		if idx < 0 || idx >= len(metadata) {
			log.Printf("Skipping %s: track number %d exceeds returned album's total tracks (%d)", file.Name, file.TrackNumber, len(metadata))
			continue
		}

		fmt.Printf("Processing Track: %s\n", file.Name)
		err := TagTrack(file, metadata[idx], cover)
		if err != nil {
			fmt.Println(err)
		}
	}
	fmt.Println("Done tagging album!")
}

func getManualQuery() SearchQuery {
	reader := bufio.NewReader(os.Stdin)
	var q SearchQuery

	fmt.Println("\n--- Manual Search ---")
	fmt.Println("1. Search by iTunes Collection ID")
	fmt.Println("2. Search by Deezer Album ID")
	fmt.Println("3. Search by UPC / Barcode")
	fmt.Println("4. Search by Artist and Album")
	fmt.Print("\nSelect an option (1-4): ")

	text, _ := reader.ReadString('\n')
	choice := strings.TrimSpace(text)

	switch choice {
	case "1":
		fmt.Print("Enter iTunes ID: ")
		idStr, _ := reader.ReadString('\n')
		q.ItunesID = strings.TrimSpace(idStr)
	case "2":
		fmt.Print("Enter Deezer ID: ")
		idStr, _ := reader.ReadString('\n')
		q.DeezerID = strings.TrimSpace(idStr)
	case "3":
		fmt.Print("Enter UPC: ")
		upc, _ := reader.ReadString('\n')
		q.UPC = strings.TrimSpace(upc)
	case "4":
		fmt.Print("Enter Artist: ")
		artist, _ := reader.ReadString('\n')
		q.Artist = strings.TrimSpace(artist)
		fmt.Print("Enter Album: ")
		album, _ := reader.ReadString('\n')
		q.Album = strings.TrimSpace(album)
	default:
		fmt.Println("Invalid choice. Try again.")
		return getManualQuery() // Recursively prompt on bad input
	}
	return q
}

func resolveMetadata(query SearchQuery) []TrackMetadata {
	// If DeezerIDFallback is ever not 0 then we know we should fallback to using Deezer for metadata retrieval
	if query.DeezerIDFallback != 0 {
		fmt.Println("Falling back to using Deezer for metadata")
		return deezerLookup(query.DeezerIDFallback)
	}

	// Prioritizes iTunes Collection ID if found
	if query.ItunesID != "" {
		id, _ := strconv.ParseInt(query.ItunesID, 10, 64)
		return iTunesLookup("", id)
	}

	// If no iTunes Collection ID is found, falls back to UPC if found
	if query.UPC != "" {
		results := iTunesSearch(query.UPC)
		if results.ResultCount == 0 {
			fmt.Println("UPC not found in iTunes database.")
			return nil
		} else if results.ResultCount == 1 {
			return iTunesLookup(query.UPC, results.Results[0].CollectionID)
		}

		// Handle multiple iTunes results
		for i, album := range results.Results {
			fmt.Printf("%d: %s - %s (%d) (%d Tracks) Link: %s\n", i+1, album.ArtistName, album.CollectionName, album.ReleaseDate.Year(), album.TrackCount, album.CollectionViewURL)
		}
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("\nEnter the number for the correct iTunes album: ")
		text, _ := reader.ReadString('\n')
		num, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || num < 1 || num > results.ResultCount {
			fmt.Println("Invalid selection.")
			return nil
		}
		return iTunesLookup(query.UPC, results.Results[num-1].CollectionID)
	}

	// If no iTunes ID or UPC is found, but there is a Deezer ID use that to get UPC
	if query.DeezerID != "" {
		fmt.Println("Using Deezer ID to fetch UPC to query iTunes...")
		id, _ := strconv.ParseInt(query.DeezerID, 10, 64)
		newQuery := SearchQuery{
			UPC: deezerUPCLookup(id),
		}
		return resolveMetadata(newQuery)
	}

	// Search Deezer using Aritst and Album name
	// Will return a list of albums, user will then be prompted to select which album result is the correct match
	// Then will use that result to get the UPC and retry the query
	if query.Artist != "" && query.Album != "" {
		albums := deezerSearch(query.Artist, query.Album)
		if len(albums.Albums) == 0 {
			fmt.Println("No matching albums found on Deezer.")
			return nil
		}

		for i, a := range albums.Albums {
			advisory := "clean"
			if a.Explicit {
				advisory = "explicit"
			}
			fmt.Printf("%d: %s - %s (%d Tracks | %s)\n", i+1, a.Artist.Name, a.Title, a.NbTracks, advisory)
		}

		reader := bufio.NewReader(os.Stdin)
		fmt.Print("\nEnter the number for the correct Deezer album: ")
		text, _ := reader.ReadString('\n')
		num, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || num < 1 || num > len(albums.Albums) {
			fmt.Println("Invalid selection.")
			return nil
		}

		// First tries to use the UPC from Deezer's database to query iTunes
		fmt.Println("Fetching UPC from Deezer to query iTunes...")
		newQuery := SearchQuery{
			UPC: deezerUPCLookup(albums.Albums[num-1].ID),
		}
		result := resolveMetadata(newQuery)

		// If that fails it will fallback to using Deezer as a last resort.
		if len(result) == 0 {
			fallbackQuery := SearchQuery{
				DeezerIDFallback: albums.Albums[num-1].ID,
			}
			result = resolveMetadata(fallbackQuery)
		}
		return result
	}
	return nil
}

func TagTrack(track AudioFile, metadata TrackMetadata, cover []byte) error {
	// Write Text Tags
	err := taglib.WriteTags(track.Path, metadata.ToTagMap(), taglib.Clear)
	if err != nil {
		return fmt.Errorf("Failed to write tags for %s: %v", track.Name, err)
	}

	// Write Cover Art (if downloaded successfully)
	if len(cover) > 0 {
		err = taglib.WriteImage(track.Path, cover)
		if err != nil {
			return fmt.Errorf("Failed to write cover art to %s: %v\n", track.Name, err)
		}
	}
	return nil
}

func deezerSearch(artist string, album string) DeezerResults {
	var result DeezerResults
	url := fmt.Sprintf("https://api.deezer.com/search/album/?q=%q&limit=10", url.QueryEscape(artist+" "+album))
	err := fetchJSON(url, result)
	if err != nil {
		log.Fatal("Failed to search album in Deezer database:", err)
	}
	return result
}

func deezerUPCLookup(id int64) string {
	var result struct {
		UPC string `json:"upc"`
	}
	url := fmt.Sprintf("https://api.deezer.com/album/%d", id)
	err := fetchJSON(url, &result)
	if err != nil {
		log.Fatal("Failed to lookup album metadata in Deezer database:", err)
	}

	return result.UPC
}

func deezerLookup(id int64) []TrackMetadata {
	var albumMetadata DeezerAlbumJSON
	var trackMetadata []DeezerTrackJSON
	var trackIDs []int64
	var url string

	url = fmt.Sprintf("https://api.deezer.com/album/%d", id)
	err := fetchJSON(url, &albumMetadata)
	if err != nil {
		log.Fatal("Failed to lookup album metadata in Deezer database:", err)
	}

	for _, trackJSON := range albumMetadata.Tracks.Data {
		trackIDs = append(trackIDs, trackJSON.ID)
	}
	if len(trackIDs) != albumMetadata.TotalTracks {
		log.Fatalf("Total track IDs (%d) does not match album's reported track count (%d)", len(trackIDs), albumMetadata.TotalTracks)
	}

	for _, trackID := range trackIDs {
		var tmp DeezerTrackJSON
		url = fmt.Sprintf("https://api.deezer.com/track/%d", trackID)
		err := fetchJSON(url, &tmp)
		if err != nil {
			log.Fatal("Failed to lookup track metadata in Deezer database:", err)
		}
		trackMetadata = append(trackMetadata, tmp)
	}
	return deezerDecode(albumMetadata, trackMetadata)
}

func iTunesSearch(UPC string) iTunesResults {
	var result iTunesResults
	url := fmt.Sprintf("https://itunes.apple.com/lookup?upc=%s&entity=album", UPC)
	err := fetchJSON(url, &result)
	if err != nil {
		log.Fatal("Error looking up album in iTunes database:", err)
	}
	return result
}

func iTunesLookup(UPC string, ID int64) []TrackMetadata {
	var result iTunesJSON
	url := fmt.Sprintf("https://itunes.apple.com/lookup?id=%d&entity=song", ID)
	err := fetchJSON(url, &result)
	if err != nil {
		log.Fatal("Error looking up album in iTunes database:", err)
	}
	return itunesDecode(result, UPC)
}

func fetchCover(URL string) ([]byte, error) {
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

// Streams data directly into target variable thus avoiding io.ReadAll and returning []byte variable
func fetchJSON(URL string, target any) error {
	resp, err := http.Get(URL)
	if err != nil {
		return fmt.Errorf("HTTP GET request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP Error code: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}

	defer resp.Body.Close()
	return nil
}

func itunesDecode(album iTunesJSON, UPC string) []TrackMetadata {
	if len(album.Results) == 0 {
		log.Fatal("Didn't get any results for album")
	}

	// Adds 1 to total track count since results includes album result along with songs
	if len(album.Results) != album.Results[0].TrackCount+1 {
		log.Fatalf("Number of returned tracks (%d) from iTunes does not match reported Track Count (%d)", len(album.Results), album.Results[0].TrackCount)
	}

	var metadata []TrackMetadata
	releaseDate := album.Results[0].ReleaseDate.Format("2006-01-02")
	highResCover := strings.Replace(album.Results[0].ArtworkURL100, "100x100", "800x800", 1)

	// Starts at index 1 because the first result from iTunes is the album collection, after that follows each track
	// Total Discs is taken from first track hence index = 1
	for i := 1; i <= album.Results[0].TrackCount; i++ {
		var tmp TrackMetadata

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
		tmp.TotalDiscs = album.Results[1].DiscCount
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
	if t.TrackExplicit == true {
		tags["ITUNESADVISORY"] = []string{"1"}
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

	fmt.Printf("\nAlbum Title: %s\nAlbum Artist: %s\nGenre: %s\nYear: %s\nCopyright: %s\nTotal Tracks: %d\nTotal Discs: %d\nUPC: %s\nCollectionID: %s\n\n", metadata[0].AlbumName, metadata[0].AlbumArtist, metadata[0].Genre, metadata[0].ReleaseDate, metadata[0].Copyright, metadata[0].TotalTracks, metadata[0].TotalDiscs, metadata[0].AlbumUPC, metadata[0].CollectionID)

	// Step 1: Collect names and find the longest filename
	for _, file := range filelist {
		s1 := file.Name
		s2 := fmt.Sprintf("%d - %s", metadata[file.TrackNumber-1].TrackNumber, metadata[file.TrackNumber-1].TrackTitle)

		if len(s1) > longest {
			longest = len(s1)
		}

		jobs = append(jobs, printJob{oldName: s1, newName: s2})
	}

	// Step 2: Print with dynamic arrow padding
	for _, job := range jobs {
		// Calculate how many extra dashes we need so the arrows align perfectly
		padLength := longest - len(job.oldName)

		// Create the arrow: space + (base dashes + extra padding dashes) + "> "
		// If it's the longest filename, padLength is 0, so it gets exactly 3 dashes
		arrow := " " + strings.Repeat("-", padLength+3) + "> "

		fmt.Println(job.oldName + arrow + job.newName)
	}
}
