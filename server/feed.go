package main

import (
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// XML structures
type Feed struct {
	XMLName    xml.Name   `xml:"feed"`
	Categories []Category `xml:"category"`
}

type Category struct {
	Name   string   `xml:"name,attr"`
	Series []Series `xml:"series"`
	Items  []Item   `xml:"item"` // For movies: one item per movie
}

type Series struct {
	Name        string   `xml:"name,attr"`
	Thumbnail   string   `xml:"thumbnail,omitempty"`
	ShortDesc   string   `xml:"shortDescription,omitempty"`
	LongDesc    string   `xml:"longDescription,omitempty"`
	Seasons     []Season `xml:"season"`
}

type Season struct {
	Name        string `xml:"name,attr"`
	Thumbnail   string `xml:"thumbnail,omitempty"`
	ShortDesc   string `xml:"shortDescription,omitempty"`
	LongDesc    string `xml:"longDescription,omitempty"`
	Items       []Item `xml:"item"`
}

type Item struct {
	ID          string    `xml:"id"`
	Title       string    `xml:"title"`
	ShortDesc   string    `xml:"shortDescription"`
	LongDesc    string    `xml:"longDescription"`
	Thumbnail   string    `xml:"thumbnail"`
	ReleaseDate string    `xml:"releaseDate"`
	Content     VideoWrap `xml:"content"`
}

type VideoWrap struct {
	Video Video `xml:"video"`
}
type Video struct {
	URL          string `xml:"url"`
	Quality      string `xml:"quality"`
	StreamFormat string `xml:"streamFormat"`
	Duration     int    `xml:"duration"`
}

func ServeFeedFromDir(root, addr string) {
	http.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		feed, err := BuildFeed(root, r.Host)
		if err != nil {
			http.Error(w, "Feed error: "+err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		enc := xml.NewEncoder(w)
		enc.Indent("", "  ")
		if err := enc.Encode(feed); err != nil {
			log.Println("XML encode error:", err)
		}
	})

	http.Handle("/", http.FileServer(http.Dir(root)))
	fmt.Printf("Serving feed and static files at http://%s/feed.xml\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func BuildFeed(root, host string) (*Feed, error) {
	feed := &Feed{}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			categoryName := entry.Name()
			categoryPath := filepath.Join(root, categoryName)
			cat := Category{Name: categoryName}

			if strings.EqualFold(categoryName, "Movies") {
				// Each subfolder in Movies is a movie
				movieDirs, _ := os.ReadDir(categoryPath)
				for _, movieDir := range movieDirs {
					if movieDir.IsDir() {
						movieName := movieDir.Name()
						moviePath := filepath.Join(categoryPath, movieName)
						item, err := buildMovieItem(moviePath, host, categoryName, movieName)
						if err == nil {
							cat.Items = append(cat.Items, item)
						} else {
							log.Println("Skipping movie:", movieName, err)
						}
					}
				}
			} else if strings.EqualFold(categoryName, "Tv Shows") || strings.EqualFold(categoryName, "TV Shows") {
				// Each subfolder in Tv Shows is a show (series)
				seriesDirs, _ := os.ReadDir(categoryPath)
				for _, seriesDir := range seriesDirs {
					if !seriesDir.IsDir() {
						continue
					}
					seriesName := seriesDir.Name()
					seriesPath := filepath.Join(categoryPath, seriesName)
					series := Series{Name: seriesName}

					// Load series-level art/description if present
					loadDescAndThumb(seriesPath, host, categoryName, seriesName, &series.ShortDesc, &series.LongDesc, &series.Thumbnail)

					// Each subfolder in a series is a season (season 1, etc)
					seasonEntries, _ := os.ReadDir(seriesPath)
					for _, seasonDir := range seasonEntries {
						if !seasonDir.IsDir() || !strings.HasPrefix(strings.ToLower(seasonDir.Name()), "season") {
							continue
						}
						seasonName := seasonDir.Name()
						seasonPath := filepath.Join(seriesPath, seasonName)
						season := Season{Name: seasonName}
						// Load season-level art/description if present
						loadDescAndThumb(seasonPath, host, categoryName, seriesName, &season.ShortDesc, &season.LongDesc, &season.Thumbnail)

						episodes, err := buildEpisodeItems(seasonPath, host, categoryName, seriesName, seasonName)
						if err != nil {
							log.Println("Error in season:", seasonName, err)
							continue
						}
						season.Items = episodes
						series.Seasons = append(series.Seasons, season)
					}
					if len(series.Seasons) > 0 {
						cat.Series = append(cat.Series, series)
					}
				}
			} else {
				// Other: fallback, try as movie folders
				subDirs, _ := os.ReadDir(categoryPath)
				for _, subDir := range subDirs {
					if subDir.IsDir() {
						subName := subDir.Name()
						subPath := filepath.Join(categoryPath, subName)
						item, err := buildMovieItem(subPath, host, categoryName, subName)
						if err == nil {
							cat.Items = append(cat.Items, item)
						}
					}
				}
			}

			feed.Categories = append(feed.Categories, cat)
		}
	}
	return feed, nil
}

// For Movies: each movie is a subfolder with media files inside
func buildMovieItem(moviePath, host, category, movieName string) (Item, error) {
	var item Item
	var shortDesc, longDesc, videoFile, thumbFile string

	files, err := os.ReadDir(moviePath)
	if err != nil {
		return item, err
	}
	for _, f := range files {
		name := f.Name()
		lower := strings.ToLower(name)
		fullPath := filepath.Join(moviePath, name)
		switch {
		case strings.HasSuffix(lower, ".mp4"):
			videoFile = name
		case strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".png"):
			thumbFile = name
		case strings.HasSuffix(lower, ".txt"):
			b, err := os.ReadFile(fullPath)
			if err == nil {
				lines := strings.SplitN(string(b), "\n", 2)
				shortDesc = strings.TrimSpace(lines[0])
				if len(lines) > 1 {
					longDesc = strings.TrimSpace(lines[1])
				}
			}
		}
	}
	if videoFile == "" {
		return item, fmt.Errorf("no video found in %s", moviePath)
	}

	vidPath := encodePath(category, movieName, videoFile)
	thumbPath := ""
	if thumbFile != "" {
		thumbPath = encodePath(category, movieName, thumbFile)
	}

	releaseDate := "1900-01-01"
	fi, err := os.Stat(filepath.Join(moviePath, videoFile))
	if err == nil {
		releaseDate = fi.ModTime().Format("2006-01-02")
	}

	duration := probeDuration(filepath.Join(moviePath, videoFile))

	item = Item{
		ID:        movieName,
		Title:     strings.TrimSuffix(videoFile, filepath.Ext(videoFile)),
		ShortDesc: shortDesc,
		LongDesc:  longDesc,
		Thumbnail: "http://" + host + "/" + thumbPath,
		ReleaseDate: releaseDate,
		Content: VideoWrap{
			Video: Video{
				URL:          "http://" + host + "/" + vidPath,
				Quality:      "HD",
				StreamFormat: "mp4",
				Duration:     duration,
			},
		},
	}
	return item, nil
}

// For TV shows: each episode is a subfolder inside a season
func buildEpisodeItems(seasonPath, host, category, series, season string) ([]Item, error) {
	entries, err := os.ReadDir(seasonPath)
	if err != nil {
		return nil, err
	}
	var items []Item
	for _, entry := range entries {
		if entry.IsDir() {
			item, err := buildEpisodeItem(filepath.Join(seasonPath, entry.Name()), host, category, series, season, entry.Name())
			if err == nil {
				items = append(items, item)
			} else {
				log.Println("Skipping episode:", entry.Name(), err)
			}
		}
	}
	return items, nil
}

// For nested episode folders (TV shows)
func buildEpisodeItem(path, host, category, series, season, episode string) (Item, error) {
	var item Item
	var shortDesc, longDesc, videoFile, thumbFile string

	files, err := os.ReadDir(path)
	if err != nil {
		return item, err
	}
	for _, f := range files {
		name := f.Name()
		lower := strings.ToLower(name)
		fullPath := filepath.Join(path, name)
		switch {
		case strings.HasSuffix(lower, ".mp4"):
			videoFile = name
		case strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".png"):
			thumbFile = name
		case strings.HasSuffix(lower, ".txt"):
			b, err := os.ReadFile(fullPath)
			if err == nil {
				lines := strings.SplitN(string(b), "\n", 2)
				shortDesc = strings.TrimSpace(lines[0])
				if len(lines) > 1 {
					longDesc = strings.TrimSpace(lines[1])
				}
			}
		}
	}
	if videoFile == "" {
		return item, fmt.Errorf("no video found in %s", path)
	}

	vidPath := encodePath(category, series, season, episode, videoFile)
	thumbPath := ""
	if thumbFile != "" {
		thumbPath = encodePath(category, series, season, episode, thumbFile)
	}

	releaseDate := "1900-01-01"
	fi, err := os.Stat(filepath.Join(path, videoFile))
	if err == nil {
		releaseDate = fi.ModTime().Format("2006-01-02")
	}

	duration := probeDuration(filepath.Join(path, videoFile))

	item = Item{
		ID:        episode,
		Title:     strings.TrimSuffix(videoFile, filepath.Ext(videoFile)),
		ShortDesc: shortDesc,
		LongDesc:  longDesc,
		Thumbnail: "http://" + host + "/" + thumbPath,
		ReleaseDate: releaseDate,
		Content: VideoWrap{
			Video: Video{
				URL:          "http://" + host + "/" + vidPath,
				Quality:      "HD",
				StreamFormat: "mp4",
				Duration:     duration,
			},
		},
	}
	return item, nil
}

// Loads description (.txt) and thumbnail (.jpg/.png) at the given path, for a series or season.
func loadDescAndThumb(basePath, host, category, series string, shortDesc *string, longDesc *string, thumbUrl *string) {
	files, err := os.ReadDir(basePath)
	if err != nil {
		return
	}
	var descFile, thumbFile string
	// Prefer: [basename].txt, [basename].jpg/.png, or any .txt/.jpg/.png if not found
	for _, f := range files {
		name := f.Name()
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".txt") && (strings.Contains(lower, strings.ToLower(series)) || descFile == "") {
			descFile = name
		} else if (strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".png")) &&
			(strings.Contains(lower, strings.ToLower(series)) || thumbFile == "") {
			thumbFile = name
		}
	}
	if descFile != "" {
		b, err := os.ReadFile(filepath.Join(basePath, descFile))
		if err == nil {
			lines := strings.SplitN(string(b), "\n", 2)
			*shortDesc = strings.TrimSpace(lines[0])
			if len(lines) > 1 {
				*longDesc = strings.TrimSpace(lines[1])
			}
		}
	}
	if thumbFile != "" {
		segments := []string{category, series, thumbFile}
		*thumbUrl = "http://" + host + "/" + encodePath(segments...)
	}
}

// encodePath encodes each path segment for a valid URL path.
func encodePath(segments ...string) string {
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return strings.Join(segments, "/")
}

// probeDuration returns the video duration in seconds if ffprobe is available, else 0.
func probeDuration(videoPath string) int {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of",
		"default=noprint_wrappers=1:nokey=1", videoPath)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Warning: Could not probe duration for %s: %v", videoPath, err)
		return 0
	}
	str := strings.TrimSpace(string(output))
	dur, err := strconv.ParseFloat(str, 64)
	if err != nil {
		log.Printf("Warning: Could not parse ffprobe output for %s: %v", videoPath, err)
		return 0
	}
	return int(dur + 0.5)
}