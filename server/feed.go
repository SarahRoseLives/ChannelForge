package main

import (
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
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
	Name      string   `xml:"name,attr"`
	Thumbnail string   `xml:"thumbnail,omitempty"`
	ShortDesc string   `xml:"shortDescription,omitempty"`
	LongDesc  string   `xml:"longDescription,omitempty"`
	Seasons   []Season `xml:"season"`
}

type Season struct {
	Name      string `xml:"name,attr"`
	Thumbnail string `xml:"thumbnail,omitempty"`
	ShortDesc string `xml:"shortDescription,omitempty"`
	LongDesc  string `xml:"longDescription,omitempty"`
	Items     []Item `xml:"item"`
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

	http.Handle("/content/", http.StripPrefix("/content/", http.FileServer(http.Dir(root))))
	fmt.Printf("Serving feed and static files at http://%s/feed.xml\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func BuildFeed(root, host string) (*Feed, error) {
	cats, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var feed Feed
	for _, c := range cats {
		if !c.IsDir() {
			continue
		}
		catName := c.Name()
		catPath := filepath.Join(root, catName)
		// If category is Movies, treat folders as movies, else treat as series
		if strings.EqualFold(catName, "movies") {
			subdirs, _ := os.ReadDir(catPath)
			var items []Item
			for _, sub := range subdirs {
				if !sub.IsDir() {
					continue
				}
				item, err := buildMovieItem(filepath.Join(catPath, sub.Name()), host, catName, sub.Name())
				if err != nil {
					log.Println("Skipping movie:", sub.Name(), err)
					continue
				}
				items = append(items, item)
			}
			feed.Categories = append(feed.Categories, Category{
				Name:  catName,
				Items: items,
			})
		} else {
			// TV Shows or other: treat each subdir as a series
			seriesDirs, _ := os.ReadDir(catPath)
			var seriesList []Series
			for _, sdir := range seriesDirs {
				if !sdir.IsDir() {
					continue
				}
				sName := sdir.Name()
				sPath := filepath.Join(catPath, sName)
				var shortDesc, longDesc, thumbUrl string
				loadDescAndThumbOrCreate(sPath, host, catName, sName, &shortDesc, &longDesc, &thumbUrl)
				seasonDirs, _ := os.ReadDir(sPath)
				var seasons []Season
				for _, sedir := range seasonDirs {
					if !sedir.IsDir() {
						continue
					}
					seasonName := sedir.Name()
					seasonPath := filepath.Join(sPath, seasonName)
					var sShort, sLong, sThumb string
					loadDescAndThumbOrCreate(seasonPath, host, catName, sName+" "+seasonName, &sShort, &sLong, &sThumb)
					eps, err := buildEpisodeItems(seasonPath, host, catName, sName, seasonName)
					if err != nil {
						log.Println("Skipping season:", seasonName, err)
						continue
					}
					seasons = append(seasons, Season{
						Name:      seasonName,
						ShortDesc: sShort,
						LongDesc:  sLong,
						Thumbnail: sThumb,
						Items:     eps,
					})
				}
				seriesList = append(seriesList, Series{
					Name:      sName,
					Thumbnail: thumbUrl,
					ShortDesc: shortDesc,
					LongDesc:  longDesc,
					Seasons:   seasons,
				})
			}
			feed.Categories = append(feed.Categories, Category{
				Name:   catName,
				Series: seriesList,
			})
		}
	}
	return &feed, nil
}

// Helper: extract a frame from video and save as a JPEG
func extractFrameAsJPG(videoPath, jpgPath string) error {
	// Example: extract frame at 10 seconds (adjust as needed)
	cmd := exec.Command("ffmpeg", "-y", "-ss", "10", "-i", videoPath, "-vframes", "1", "-q:v", "2", jpgPath)
	return cmd.Run()
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

	// If thumbFile doesn't exist, create one from video
	if thumbFile == "" && videoFile != "" {
		thumbFile = "thumb.jpg"
		thumbPath := filepath.Join(moviePath, thumbFile)
		videoPath := filepath.Join(moviePath, videoFile)
		_ = extractFrameAsJPG(videoPath, thumbPath)
	}

	// If desc.txt doesn't exist, create one
	if shortDesc == "" && longDesc == "" {
		shortDesc = movieName
		longDesc = movieName
		descPath := filepath.Join(moviePath, "desc.txt")
		_ = os.WriteFile(descPath, []byte(shortDesc+"\n"+longDesc), 0644)
	}

	vidPath := encodeContentPath(category, movieName, videoFile)
	thumbPath := encodeContentPath(category, movieName, thumbFile)

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
		Thumbnail: "http://" + host + "/content/" + thumbPath,
		ReleaseDate: releaseDate,
		Content: VideoWrap{
			Video: Video{
				URL:          "http://" + host + "/content/" + vidPath,
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

	// If thumbFile doesn't exist, create one from video
	if thumbFile == "" && videoFile != "" {
		thumbFile = "thumb.jpg"
		thumbPath := filepath.Join(path, thumbFile)
		videoPath := filepath.Join(path, videoFile)
		_ = extractFrameAsJPG(videoPath, thumbPath)
	}

	// If desc.txt doesn't exist, create one
	if shortDesc == "" && longDesc == "" {
		shortDesc = episode
		longDesc = episode
		descPath := filepath.Join(path, "desc.txt")
		_ = os.WriteFile(descPath, []byte(shortDesc+"\n"+longDesc), 0644)
	}

	vidPath := encodeContentPath(category, series, season, episode, videoFile)
	thumbPath := encodeContentPath(category, series, season, episode, thumbFile)

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
		Thumbnail: "http://" + host + "/content/" + thumbPath,
		ReleaseDate: releaseDate,
		Content: VideoWrap{
			Video: Video{
				URL:          "http://" + host + "/content/" + vidPath,
				Quality:      "HD",
				StreamFormat: "mp4",
				Duration:     duration,
			},
		},
	}
	return item, nil
}

// Loads description (.txt) and thumbnail (.jpg/.png) at the given path, for a series or season.
// If the files do not exist, create them with defaults matching the name.
// For series/seasons, if no video is present, makes a default color JPEG.
func loadDescAndThumbOrCreate(basePath, host, category, name string, shortDesc *string, longDesc *string, thumbUrl *string) {
	files, err := os.ReadDir(basePath)
	if err != nil {
		// Try to create the directory if missing
		_ = os.MkdirAll(basePath, 0755)
		files = []os.DirEntry{}
	}

	var descFile, thumbFile, videoFile string
	for _, f := range files {
		lower := strings.ToLower(f.Name())
		if strings.HasSuffix(lower, ".txt") && (strings.Contains(lower, strings.ToLower(name)) || descFile == "") {
			descFile = f.Name()
		} else if (strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".png")) &&
			(strings.Contains(lower, strings.ToLower(name)) || thumbFile == "") {
			thumbFile = f.Name()
		} else if strings.HasSuffix(lower, ".mp4") && videoFile == "" {
			videoFile = f.Name()
		}
	}
	// If desc.txt doesn't exist, create it
	if descFile == "" {
		descFile = "desc.txt"
		descPath := filepath.Join(basePath, descFile)
		*shortDesc = name
		*longDesc = name
		_ = os.WriteFile(descPath, []byte(name+"\n"+name), 0644)
	} else {
		b, err := os.ReadFile(filepath.Join(basePath, descFile))
		if err == nil {
			lines := strings.SplitN(string(b), "\n", 2)
			*shortDesc = strings.TrimSpace(lines[0])
			if len(lines) > 1 {
				*longDesc = strings.TrimSpace(lines[1])
			}
		}
	}
	// If thumb does not exist, create it (try to use a video frame, else make a default color jpg)
	if thumbFile == "" {
		thumbFile = "thumb.jpg"
		thumbPath := filepath.Join(basePath, thumbFile)
		if videoFile != "" {
			videoPath := filepath.Join(basePath, videoFile)
			_ = extractFrameAsJPG(videoPath, thumbPath)
		} else {
			createDefaultJPG(thumbPath, name)
		}
	}
	segments := []string{category, name, thumbFile}
	*thumbUrl = "http://" + host + "/content/" + encodeContentPath(segments...)
}

// encodeContentPath encodes each path segment for a valid URL path, used under /content/
func encodeContentPath(segments ...string) string {
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

// Helper: does a directory have a video file (used for hybrid categories)
func dirHasVideo(dir string) bool {
	files, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(strings.ToLower(f.Name()), ".mp4") {
			return true
		}
	}
	return false
}

// Utility for removing a directory tree (used in admin UI for delete actions)
func removeTree(path string) error {
	return os.RemoveAll(path)
}

// createDefaultJPG creates a plain JPEG image with the given name as an overlay/text (optional)
func createDefaultJPG(path, name string) {
	const width, height = 640, 360
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Fill with a flat color (e.g. blue)
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{0x4a, 0x90, 0xe2, 0xff}}, image.Point{}, draw.Src)
	// Optionally, add more graphical info or text (for now, just color)
	// (To add text, use a font lib such as freetype, but we avoid extra deps here)
	// Save to file
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	_ = jpeg.Encode(f, img, &jpeg.Options{Quality: 80})
}