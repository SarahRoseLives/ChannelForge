package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"golang.org/x/crypto/bcrypt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const userFile = "webuser.json"

type User struct {
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
}

var (
	sessionMu sync.Mutex
	sessions  = make(map[string]string) // sessionToken -> username
)

var css = `<style>
body {
  background: #252850;
  font-family: 'Segoe UI', Arial, sans-serif;
  color: #f5f6fa;
  display: flex;
  flex-direction: column;
  min-height: 100vh;
  margin: 0;
}
nav { padding: 1em 2em 0 2em; }
nav a { color: #97b4ff; margin-right: 1.5em; text-decoration: none; font-weight: 500; }
nav a:hover { text-decoration: underline; }
.card {
  background: rgba(49,51,100,0.97);
  padding: 2.5em 2em 2em 2em;
  border-radius: 18px;
  box-shadow: 0 2px 16px #0003;
  min-width: 320px;
  max-width: 95vw;
  margin: 2em auto 2em auto;
  backdrop-filter: blur(2.5px);
}
h2 {
  margin-top: 0;
  margin-bottom: 1.2em;
  text-align: center;
  font-weight: 600;
  letter-spacing: 0.04em;
}
label {
  display: block;
  margin: 1em 0 0.5em;
  font-size: 1.1em;
}
input[type="text"], input[type="password"], textarea, select {
  width: 100%;
  padding: 0.7em;
  margin-bottom: 1.3em;
  border-radius: 8px;
  border: none;
  font-size: 1em;
  background: #222348;
  color: #f5f6fa;
  outline: none;
  box-shadow: 0 0 0 2px #222348;
  transition: box-shadow 0.15s;
}
input[type="text"]:focus, input[type="password"]:focus, textarea:focus, select:focus {
  box-shadow: 0 0 0 2px #5976ff;
}
button, .btn {
  background: linear-gradient(90deg, #5976ff 0%, #6c8fff 100%);
  color: #fff;
  border: none;
  border-radius: 8px;
  padding: 0.55em 1.5em;
  font-size: 1.04em;
  font-weight: bold;
  cursor: pointer;
  box-shadow: 0 2px 6px #0002;
  transition: background 0.15s, filter 0.15s;
  margin-right: 0.7em;
  margin-top: 0.2em;
  margin-bottom: 0.2em;
  display: inline-block;
}
button:hover, .btn:hover { filter: brightness(0.93); }
.error {
  color: #ff6c6c;
  margin-bottom: 1em;
  text-align: center;
  font-weight: bold;
}
input:-webkit-autofill, input:-webkit-autofill:focus {
  -webkit-box-shadow: 0 0 0 100px #222348 inset !important;
  -webkit-text-fill-color: #f5f6fa !important;
  caret-color: #f5f6fa !important;
  border-radius: 8px;
}
input:-webkit-autofill::first-line { font-family: 'Segoe UI', Arial, sans-serif; }
th, td { padding: 0.5em 1em; text-align: left; }
tr:nth-child(even) { background: #2a2c58; }
tr:nth-child(odd) { background: #23244a; }
table { width: 100%; border-collapse: collapse; margin-bottom: 2em;}
a.action { color: #ffb66c; font-weight: bold; margin-left: 0.7em; }
a.action:hover { text-decoration: underline; }
</style>`

var (
	setupPage = template.Must(template.New("setup").Parse(`
<html><head><title>Setup Admin Account</title>` + css + `</head><body>
<div class="card">
<h2>Setup Your Admin Account</h2>
<form method="POST" action="/setup">
  <label>Username <input name="username" required autocomplete="username"></label>
  <label>Password <input type="password" name="password" required autocomplete="new-password"></label>
  <button type="submit">Create Account</button>
</form>
</div>
</body></html>`))

	loginPage = template.Must(template.New("login").Parse(`
<html><head><title>Login</title>` + css + `</head><body>
<div class="card">
<h2>Sign In</h2>
{{if .Error}}<div class="error">{{.Error}}</div>{{end}}
<form method="POST" action="/login">
  <label>Username <input name="username" required autocomplete="username"></label>
  <label>Password <input type="password" name="password" required autocomplete="current-password"></label>
  <button type="submit">Login</button>
</form>
</div>
</body></html>`))
)

var adminPage = template.Must(template.New("admin").Funcs(template.FuncMap{
	"base": filepath.Base,
}).Parse(`
<html><head><title>Admin Panel</title>` + css + `</head><body>
<nav>
  <a href="/admin">Dashboard</a>
  <a href="/admin/newcat">+ New Category</a>
  <a href="/logout">Logout</a>
</nav>
<div class="card">
<h2>Content Overview</h2>
{{if .Error}}<div class="error">{{.Error}}</div>{{end}}
{{if .Categories}}
  <table>
    <tr><th>Category</th><th>Type</th><th>Actions</th></tr>
    {{range .Categories}}
      <tr>
        <td>{{.Name}}</td>
        <td>
          {{.Type}}
        </td>
        <td>
          <a href="/admin/cat/{{.Name}}" class="btn">Browse/Edit</a>
          <form method="POST" action="/admin/delcat" style="display:inline">
            <input type="hidden" name="category" value="{{.Name}}">
            <button type="submit" class="btn" onclick="return confirm('Delete category {{.Name}}? This cannot be undone.')">Delete</button>
          </form>
        </td>
      </tr>
    {{end}}
  </table>
{{else}}
  <p>No categories found. <a href="/admin/newcat">Create one</a></p>
{{end}}
</div>
</body></html>
`))

var newCatPage = template.Must(template.New("newcat").Parse(`
<html><head><title>New Category</title>` + css + `</head><body>
<nav>
  <a href="/admin">Dashboard</a>
  <a href="/logout">Logout</a>
</nav>
<div class="card">
<h2>New Category</h2>
<form method="POST" action="/admin/newcat">
  <label>Category Name
    <input name="catname" required maxlength="60" pattern="[^/]+" placeholder="e.g. Movies, TV Shows, Documentaries">
  </label>
  <button type="submit">Create Category</button>
</form>
</div>
</body></html>
`))

var catPage = template.Must(template.New("cat").Funcs(template.FuncMap{
	"base": filepath.Base,
}).Parse(`
<html><head><title>{{.Category}} - Admin</title>` + css + `</head><body>
<nav>
  <a href="/admin">Dashboard</a>
  <a href="/admin/newcat">+ New Category</a>
  <a href="/logout">Logout</a>
</nav>
<div class="card">
<h2>Category: {{.Category}}</h2>
{{if .Error}}<div class="error">{{.Error}}</div>{{end}}
{{if .IsMovies}}
  <h3>Add Movie</h3>
  <form method="POST" action="/admin/cat/{{.Category}}/upload" enctype="multipart/form-data">
    <label>Movie Name <input name="moviename" required maxlength="60"></label>
    <label>Short Description <input name="shortdesc" maxlength="200"></label>
    <label>Long Description <textarea name="longdesc" rows="3"></textarea></label>
    <label>Video File (.mp4) <input type="file" name="video" accept="video/mp4" required></label>
    <label>Thumbnail (jpg/png, optional) <input type="file" name="thumb" accept="image/*"></label>
    <button type="submit">Add Movie</button>
  </form>
  <h3>Movies</h3>
  {{if .Movies}}
    <table>
      <tr><th>Name</th><th>Actions</th></tr>
      {{range .Movies}}
        <tr>
          <td>{{.}}</td>
          <td>
            <form method="POST" action="/admin/cat/{{$.Category}}/delmovie" style="display:inline">
              <input type="hidden" name="moviename" value="{{.}}">
              <button type="submit" class="btn" onclick="return confirm('Delete movie {{.}}?')">Delete</button>
            </form>
          </td>
        </tr>
      {{end}}
    </table>
  {{else}}
    <p>No movies found.</p>
  {{end}}
{{else}}
  <h3>Add Series</h3>
  <form method="POST" action="/admin/cat/{{.Category}}/newseries">
    <label>Series Name <input name="seriesname" required maxlength="60"></label>
    <button type="submit">Create Series</button>
  </form>
  <h3>Series</h3>
  {{if .Series}}
    <table>
      <tr><th>Name</th><th>Actions</th></tr>
      {{range .Series}}
        <tr>
          <td>{{.}}</td>
          <td>
            <a href="/admin/cat/{{$.Category}}/series/{{.}}" class="btn">Browse/Edit</a>
            <form method="POST" action="/admin/cat/{{$.Category}}/delseries" style="display:inline">
              <input type="hidden" name="seriesname" value="{{.}}">
              <button type="submit" class="btn" onclick="return confirm('Delete series {{.}}?')">Delete</button>
            </form>
          </td>
        </tr>
      {{end}}
    </table>
  {{else}}
    <p>No series found.</p>
  {{end}}
{{end}}
</div>
</body></html>
`))

var seriesPage = template.Must(template.New("series").Parse(`
<html><head><title>{{.Series}} - {{.Category}}</title>` + css + `</head><body>
<nav>
  <a href="/admin">Dashboard</a>
  <a href="/admin/cat/{{.Category}}">Back to {{.Category}}</a>
  <a href="/logout">Logout</a>
</nav>
<div class="card">
<h2>Series: {{.Series}}</h2>
{{if .Error}}<div class="error">{{.Error}}</div>{{end}}
<h3>Add Season</h3>
<form method="POST" action="/admin/cat/{{.Category}}/series/{{.Series}}/newseason">
  <label>Season Name <input name="seasonname" required maxlength="60" placeholder="e.g. Season 1"></label>
  <button type="submit">Create Season</button>
</form>
<h3>Seasons</h3>
{{if .Seasons}}
  <table>
    <tr><th>Name</th><th>Actions</th></tr>
    {{range .Seasons}}
      <tr>
        <td>{{.}}</td>
        <td>
          <a href="/admin/cat/{{$.Category}}/series/{{$.Series}}/season/{{.}}" class="btn">Browse/Edit</a>
          <form method="POST" action="/admin/cat/{{$.Category}}/series/{{$.Series}}/delseason" style="display:inline">
            <input type="hidden" name="seasonname" value="{{.}}">
            <button type="submit" class="btn" onclick="return confirm('Delete season {{.}}?')">Delete</button>
          </form>
        </td>
      </tr>
    {{end}}
  </table>
{{else}}
  <p>No seasons found.</p>
{{end}}
</div>
</body></html>
`))

var seasonPage = template.Must(template.New("season").Parse(`
<html><head><title>{{.Season}} - {{.Series}} - {{.Category}}</title>` + css + `</head><body>
<nav>
  <a href="/admin">Dashboard</a>
  <a href="/admin/cat/{{.Category}}">Back to {{.Category}}</a>
  <a href="/admin/cat/{{.Category}}/series/{{.Series}}">Back to {{.Series}}</a>
  <a href="/logout">Logout</a>
</nav>
<div class="card">
<h2>{{.Season}} ({{.Series}})</h2>
{{if .Error}}<div class="error">{{.Error}}</div>{{end}}
<h3>Add Episode</h3>
<form method="POST" action="/admin/cat/{{.Category}}/series/{{.Series}}/season/{{.Season}}/uploadep" enctype="multipart/form-data">
  <label>Episode Name <input name="epname" required maxlength="60"></label>
  <label>Short Description <input name="shortdesc" maxlength="200"></label>
  <label>Long Description <textarea name="longdesc" rows="3"></textarea></label>
  <label>Video File (.mp4) <input type="file" name="video" accept="video/mp4" required></label>
  <label>Thumbnail (jpg/png, optional) <input type="file" name="thumb" accept="image/*"></label>
  <button type="submit">Add Episode</button>
</form>
<h3>Episodes</h3>
{{if .Episodes}}
  <table>
    <tr><th>Name</th><th>Actions</th></tr>
    {{range .Episodes}}
      <tr>
        <td>{{.}}</td>
        <td>
          <form method="POST" action="/admin/cat/{{$.Category}}/series/{{$.Series}}/season/{{$.Season}}/delepisode" style="display:inline">
            <input type="hidden" name="epname" value="{{.}}">
            <button type="submit" class="btn" onclick="return confirm('Delete episode {{.}}?')">Delete</button>
          </form>
        </td>
      </tr>
    {{end}}
  </table>
{{else}}
  <p>No episodes found.</p>
{{end}}
</div>
</body></html>
`))

func StartWebServer(addr string, rootDir string) {
	http.HandleFunc("/admin", requireLogin(func(w http.ResponseWriter, r *http.Request) {
		cats, err := listCategories(rootDir)
		typeinfo := func(name string) string {
			n := strings.ToLower(name)
			if n == "movies" {
				return "Movies"
			} else if n == "tv shows" || n == "tvshows" {
				return "TV Shows"
			}
			return "Other"
		}
		var out []struct {
			Name string
			Type string
		}
		for _, c := range cats {
			out = append(out, struct{ Name, Type string }{c, typeinfo(c)})
		}
		adminPage.Execute(w, map[string]interface{}{"Categories": out, "Error": err})
	}))

	http.HandleFunc("/admin/newcat", requireLogin(newCatHandler(rootDir)))
	http.HandleFunc("/admin/delcat", requireLogin(delCatHandler(rootDir)))

	http.HandleFunc("/admin/cat/", requireLogin(catHandler(rootDir)))

	http.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		feed, err := BuildFeed(rootDir, r.Host)
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

	// Auth routes
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/setup", setupHandler)
	http.HandleFunc("/", rootHandler)

	// Serve all files under /content/
	fs := http.FileServer(http.Dir(rootDir))
	http.Handle("/content/", http.StripPrefix("/content/", fs))

	log.Printf("Web server running at: http://%s/", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// ==== ADMIN HANDLERS ====

func newCatHandler(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			newCatPage.Execute(w, nil)
			return
		}
		if r.Method == "POST" {
			name := strings.TrimSpace(r.FormValue("catname"))
			if len(name) < 2 || strings.ContainsAny(name, "/\\") {
				newCatPage.Execute(w, map[string]string{"Error": "Invalid name"})
				return
			}
			full := filepath.Join(root, name)
			if _, err := os.Stat(full); err == nil {
				newCatPage.Execute(w, map[string]string{"Error": "Category exists"})
				return
			}
			if err := os.MkdirAll(full, 0755); err != nil {
				newCatPage.Execute(w, map[string]string{"Error": err.Error()})
				return
			}
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
			return
		}
		http.Error(w, "Method not allowed", 405)
	}
}

func delCatHandler(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		name := strings.TrimSpace(r.FormValue("category"))
		if name == "" {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
			return
		}
		full := filepath.Join(root, name)
		if err := os.RemoveAll(full); err != nil {
			adminPage.Execute(w, map[string]interface{}{"Error": "Failed to delete: " + err.Error()})
			return
		}
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}
}

func catHandler(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// /admin/cat/{cat}
		path := strings.TrimPrefix(r.URL.Path, "/admin/cat/")
		if path == "" || strings.Contains(path, "/") {
			// This case should be handled by the catRouter, but as a fallback:
			http.NotFound(w, r)
			return
		}
		cat := path
		catPath := filepath.Join(root, cat)
		fi, err := os.Stat(catPath)
		if err != nil || !fi.IsDir() {
			adminPage.Execute(w, map[string]interface{}{"Error": "Category not found"})
			return
		}
		isMovies := strings.EqualFold(cat, "movies")
		if isMovies {
			// Movie folders
			movies, _ := listSubDirs(catPath)
			catPage.Execute(w, map[string]interface{}{
				"Category": cat,
				"IsMovies": true,
				"Movies":   movies,
			})
		} else {
			// Series folders
			series, _ := listSubDirs(catPath)
			catPage.Execute(w, map[string]interface{}{
				"Category": cat,
				"IsMovies": false,
				"Series":   series,
			})
		}
	}
}

// Handles subroutes under /admin/cat/{cat}/...
func catRouter(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		trim := strings.TrimPrefix(r.URL.Path, "/admin/cat/")
		parts := strings.Split(trim, "/")
		if len(parts) < 2 {
			http.NotFound(w, r) // Let catHandler deal with /admin/cat/{cat}
			return
		}

		cat := parts[0]
		action := parts[1]
		catPath := filepath.Join(root, cat)

		// /admin/cat/{cat}/upload or /admin/cat/{cat}/delmovie
		if len(parts) == 2 {
			if strings.EqualFold(cat, "movies") {
				if action == "upload" && r.Method == "POST" {
					handleMovieUpload(catPath, w, r, cat)
					return
				}
				if action == "delmovie" && r.Method == "POST" {
					movie := r.FormValue("moviename")
					if movie != "" {
						moviePath := filepath.Join(catPath, movie)
						_ = os.RemoveAll(moviePath)
					}
					http.Redirect(w, r, "/admin/cat/"+cat, http.StatusSeeOther)
					return
				}
			} else { // It's a series category
				if action == "newseries" && r.Method == "POST" {
					name := strings.TrimSpace(r.FormValue("seriesname"))
					if len(name) > 1 && !strings.ContainsAny(name, "/\\") {
						full := filepath.Join(catPath, name)
						_ = os.MkdirAll(full, 0755)
					}
					http.Redirect(w, r, "/admin/cat/"+cat, http.StatusSeeOther)
					return
				}
				if action == "delseries" && r.Method == "POST" {
					series := r.FormValue("seriesname")
					if series != "" {
						seriesPath := filepath.Join(catPath, series)
						_ = os.RemoveAll(seriesPath)
					}
					http.Redirect(w, r, "/admin/cat/"+cat, http.StatusSeeOther)
					return
				}
			}
		}

		// Handle deeper routes like /admin/cat/{cat}/series/{series_name}/...
		if len(parts) >= 3 && action == "series" {
			series := parts[2]
			seriesPath := filepath.Join(catPath, series)

			if len(parts) == 3 { // It's the series page itself
				seasons, _ := listSubDirs(seriesPath)
				seriesPage.Execute(w, map[string]interface{}{
					"Category": cat,
					"Series":   series,
					"Seasons":  seasons,
				})
				return
			}

			// Actions on a series, e.g. /.../newseason
			seriesAction := parts[3]
			if r.Method == "POST" {
				switch seriesAction {
				case "newseason":
					name := strings.TrimSpace(r.FormValue("seasonname"))
					if len(name) > 1 && !strings.ContainsAny(name, "/\\") {
						_ = os.MkdirAll(filepath.Join(seriesPath, name), 0755)
					}
					http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
					return
				case "delseason":
					season := r.FormValue("seasonname")
					if season != "" {
						_ = os.RemoveAll(filepath.Join(seriesPath, season))
					}
					http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
					return
				}
			}

			// Handle season-level routes /.../season/{season_name}/...
			if len(parts) >= 5 && seriesAction == "season" {
				season := parts[4]
				seasonPath := filepath.Join(seriesPath, season)

				if len(parts) == 5 { // It's the season page
					episodes, _ := listSubDirs(seasonPath)
					seasonPage.Execute(w, map[string]interface{}{
						"Category": cat,
						"Series":   series,
						"Season":   season,
						"Episodes": episodes,
					})
					return
				}

				episodeAction := parts[5]
				if r.Method == "POST" {
					switch episodeAction {
					case "uploadep":
						handleEpisodeUpload(seasonPath, w, r, cat, series, season)
						return
					case "delepisode":
						ep := r.FormValue("epname")
						if ep != "" {
							_ = os.RemoveAll(filepath.Join(seasonPath, ep))
						}
						http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
						return
					}
				}
			}
		}
		// If no route was matched, fall through to the catHandler for the base category page
		catHandler(root)(w, r)
	}
}

// ==== UPLOAD HANDLERS ====

func handleMovieUpload(catPath string, w http.ResponseWriter, r *http.Request, cat string) {
	if err := r.ParseMultipartForm(100 << 20); err != nil { // 100MB
		catPage.Execute(w, map[string]interface{}{"Category": cat, "IsMovies": true, "Error": "Error parsing form"})
		return
	}
	name := strings.TrimSpace(r.FormValue("moviename"))
	short := r.FormValue("shortdesc")
	long := r.FormValue("longdesc")
	if name == "" {
		catPage.Execute(w, map[string]interface{}{"Category": cat, "IsMovies": true, "Error": "Movie name required"})
		return
	}
	dir := filepath.Join(catPath, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		catPage.Execute(w, map[string]interface{}{"Category": cat, "IsMovies": true, "Error": "Failed to create dir"})
		return
	}
	// video file
	video, vhead, err := r.FormFile("video")
	if err != nil {
		catPage.Execute(w, map[string]interface{}{"Category": cat, "IsMovies": true, "Error": "Missing video"})
		return
	}
	defer video.Close()
	vpath := filepath.Join(dir, vhead.Filename)
	if err := saveUploadedFile(video, vpath); err != nil {
		catPage.Execute(w, map[string]interface{}{"Category": cat, "IsMovies": true, "Error": "Failed to save video: " + err.Error()})
		return
	}

	// thumb (optional)
	if thumb, thead, err := r.FormFile("thumb"); err == nil && thead.Filename != "" {
		defer thumb.Close()
		tpath := filepath.Join(dir, thead.Filename)
		if err := saveUploadedFile(thumb, tpath); err != nil {
			catPage.Execute(w, map[string]interface{}{"Category": cat, "IsMovies": true, "Error": "Failed to save thumbnail: " + err.Error()})
			return
		}
	}
	// desc (txt)
	if short != "" || long != "" {
		txt := short + "\n" + long
		if err := os.WriteFile(filepath.Join(dir, "desc.txt"), []byte(txt), 0644); err != nil {
			catPage.Execute(w, map[string]interface{}{"Category": cat, "IsMovies": true, "Error": "Failed to save description: " + err.Error()})
			return
		}
	}
	http.Redirect(w, r, "/admin/cat/"+cat, http.StatusSeeOther)
}

func handleEpisodeUpload(seasonPath string, w http.ResponseWriter, r *http.Request, cat, ser, season string) {
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		seasonPage.Execute(w, map[string]interface{}{"Category": cat, "Series": ser, "Season": season, "Error": "Form error"})
		return
	}
	epname := strings.TrimSpace(r.FormValue("epname"))
	short := r.FormValue("shortdesc")
	long := r.FormValue("longdesc")
	if epname == "" {
		seasonPage.Execute(w, map[string]interface{}{"Category": cat, "Series": ser, "Season": season, "Error": "Episode name required"})
		return
	}
	dir := filepath.Join(seasonPath, epname)
	if err := os.MkdirAll(dir, 0755); err != nil {
		seasonPage.Execute(w, map[string]interface{}{"Category": cat, "Series": ser, "Season": season, "Error": "Failed to create dir"})
		return
	}
	video, vhead, err := r.FormFile("video")
	if err != nil {
		seasonPage.Execute(w, map[string]interface{}{"Category": cat, "Series": ser, "Season": season, "Error": "Missing video"})
		return
	}
	defer video.Close()
	vpath := filepath.Join(dir, vhead.Filename)
	if err := saveUploadedFile(video, vpath); err != nil {
		seasonPage.Execute(w, map[string]interface{}{"Category": cat, "Series": ser, "Season": season, "Error": "Failed to save video: " + err.Error()})
		return
	}
	if thumb, thead, err := r.FormFile("thumb"); err == nil && thead.Filename != "" {
		defer thumb.Close()
		tpath := filepath.Join(dir, thead.Filename)
		if err := saveUploadedFile(thumb, tpath); err != nil {
			seasonPage.Execute(w, map[string]interface{}{"Category": cat, "Series": ser, "Season": season, "Error": "Failed to save thumbnail: " + err.Error()})
			return
		}
	}
	if short != "" || long != "" {
		txt := short + "\n" + long
		if err := os.WriteFile(filepath.Join(dir, "desc.txt"), []byte(txt), 0644); err != nil {
			seasonPage.Execute(w, map[string]interface{}{"Category": cat, "Series": ser, "Season": season, "Error": "Failed to save description: " + err.Error()})
			return
		}
	}
	http.Redirect(w, r, "/admin/cat/"+cat+"/series/"+ser+"/season/"+season, http.StatusSeeOther)
}

func saveUploadedFile(f multipart.File, target string) error {
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, f)
	return err
}

// ==== HELPERS ====

func listCategories(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func listSubDirs(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// ==== AUTH BOILERPLATE (unchanged) ====

func userExists() bool {
	_, err := os.Stat(userFile)
	return err == nil
}
func saveUser(u User) error {
	f, err := os.Create(userFile)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(u)
}
func loadUser() (*User, error) {
	f, err := os.Open(userFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var u User
	err = json.NewDecoder(f).Decode(&u)
	return &u, err
}
func rootHandler(w http.ResponseWriter, r *http.Request) {
	if !userExists() {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
func setupHandler(w http.ResponseWriter, r *http.Request) {
	if userExists() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	switch r.Method {
	case "GET":
		setupPage.Execute(w, nil)
	case "POST":
		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")
		if len(username) < 3 || len(password) < 5 {
			http.Error(w, "Username or password too short.", 400)
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Internal error", 500)
			return
		}
		u := User{Username: username, PasswordHash: string(hash)}
		if err := saveUser(u); err != nil {
			http.Error(w, "Failed to save user: "+err.Error(), 500)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	default:
		http.Error(w, "Method not allowed", 405)
	}
}
func loginHandler(w http.ResponseWriter, r *http.Request) {
	if !userExists() {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	switch r.Method {
	case "GET":
		loginPage.Execute(w, nil)
	case "POST":
		username := r.FormValue("username")
		password := r.FormValue("password")
		u, err := loadUser()
		if err != nil || username != u.Username {
			loginPage.Execute(w, map[string]string{"Error": "Invalid username or password"})
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
			loginPage.Execute(w, map[string]string{"Error": "Invalid username or password"})
			return
		}
		token := newSessionToken()
		sessionMu.Lock()
		sessions[token] = username
		sessionMu.Unlock()
		http.SetCookie(w, &http.Cookie{
			Name:     "session",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	default:
		http.Error(w, "Method not allowed", 405)
	}
}
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		sessionMu.Lock()
		delete(sessions, cookie.Value)
		sessionMu.Unlock()
		http.SetCookie(w, &http.Cookie{
			Name:     "session",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
func checkSession(r *http.Request) string {
	cookie, err := r.Cookie("session")
	if err != nil {
		return ""
	}
	sessionMu.Lock()
	defer sessionMu.Unlock()
	return sessions[cookie.Value]
}
func newSessionToken() string {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(b)
}
func requireLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if username := checkSession(r); username != "" {
			next(w, r)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

// Dummy BuildFeed function to allow compilation
type RSS struct {
	XMLName xml.Name `xml:"rss"`
}

