package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type CinemaItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Year        string   `json:"year"`
	Poster      string   `json:"poster"`
	Backdrop    string   `json:"backdrop"`
	Logo        string   `json:"logo,omitempty"`
	ImdbRating  string   `json:"imdbRating"`
	Description string   `json:"description"`
	Genres      []string `json:"genres"`
}

type CinemaEpisode struct {
	ID        string `json:"id"`
	Season    int    `json:"season"`
	Episode   int    `json:"episode"`
	Title     string `json:"title"`
	Thumbnail string `json:"thumbnail"`
	Overview  string `json:"overview"`
	Released  string `json:"released"`
}

type CinemaDeepMeta struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Type        string          `json:"type"`
	Year        string          `json:"year"`
	Poster      string          `json:"poster"`
	Background  string          `json:"background"`
	Logo        string          `json:"logo,omitempty"`
	ImdbRating  string          `json:"imdbRating"`
	Description string          `json:"description"`
	Genres      []string        `json:"genres"`
	Cast        []string        `json:"cast"`
	Runtime     string          `json:"runtime"`
	Videos      []CinemaEpisode `json:"videos"`
}

type CinemaStream struct {
	Name          string   `json:"name"`
	Title         string   `json:"title"`
	InfoHash      string   `json:"infoHash,omitempty"`
	FileIdx       *int     `json:"fileIdx,omitempty"`
	URL           string   `json:"url,omitempty"`
	AudioURL      string   `json:"audioUrl,omitempty"`
	Sources       []string `json:"sources,omitempty"`
	Quality       string   `json:"quality"`
	Seeders       int      `json:"seeders"`
	SizeText      string   `json:"sizeText"`
	ReleaseTitle  string   `json:"releaseTitle"`
	CodecTags     string   `json:"codecTags"`
	HasArabicHint bool     `json:"hasArabicHint"`
	IsDirect      bool     `json:"isDirect"`
	Score         int      `json:"score"`
}

type SubtitleTrack struct {
	ID        string `json:"id"`
	LangCode  string `json:"langCode"`
	LangLabel string `json:"langLabel"`
	URL       string `json:"url"`
}

type AudioTrack struct {
	AudioIdx  int    `json:"audioIdx"`
	LangCode  string `json:"langCode"`
	LangLabel string `json:"langLabel"`
	Codec     string `json:"codec"`
	Title     string `json:"title,omitempty"`
}

var (
	pcHttpClient = &http.Client{
		Timeout: 9 * time.Second,
	}
	pcFfmpegPath     string
	pcFfmpegProcsMu  sync.Mutex
	pcFfmpegProcs    = make(map[string]*exec.Cmd)
	pcStateMu        sync.Mutex
	pcStateFile      string
	pcActiveToken    string
	pcListenPort     int
	pcWifiProfile    = "balanced" // "balanced" (Meh Wi-Fi optimized) or "fiber4k"
)

func detectFfmpegBinary() string {
	if pcFfmpegPath != "" {
		return pcFfmpegPath
	}
	// 1. Check right next to NOIR.exe
	if exe, err := os.Executable(); err == nil {
		cand := filepath.Join(filepath.Dir(exe), "ffmpeg.exe")
		if _, err := os.Stat(cand); err == nil {
			pcFfmpegPath = cand
			return cand
		}
		candUnix := filepath.Join(filepath.Dir(exe), "ffmpeg")
		if _, err := os.Stat(candUnix); err == nil {
			pcFfmpegPath = candUnix
			return candUnix
		}
	}
	// 2. Check %LOCALAPPDATA%\NOIR\ffmpeg.exe
	if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
		cand := filepath.Join(localApp, "NOIR", "ffmpeg.exe")
		if _, err := os.Stat(cand); err == nil {
			pcFfmpegPath = cand
			return cand
		}
	}
	// 3. Check PATH
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		pcFfmpegPath = p
		return p
	}
	// 4. Check common imageio-ffmpeg site-packages location
	matches, _ := filepath.Glob(`C:\Users\*\AppData\Local\Programs\Python\*\Lib\site-packages\imageio_ffmpeg\binaries\ffmpeg*.exe`)
	if len(matches) > 0 {
		pcFfmpegPath = matches[0]
		return matches[0]
	}
	return ""
}

func curatedArabicCartoons() []CinemaItem {
	return []CinemaItem{
		{
			ID: "tt3718778", Name: "Over the Garden Wall (خلف حائط الحديقة)", Type: "series", Year: "2014",
			Poster: "https://images.metahub.space/poster/small/tt3718778/img",
			Backdrop: "https://images.metahub.space/background/medium/tt3718778/img",
			ImdbRating: "8.8", Description: "Two brothers lose themselves in a strange forest filled with wandering souls. Includes official Cartoon Network Arabic dub & 1080p BluRay episodes.",
			Genres: []string{"Animation", "Fantasy", "Adventure"},
		},
		{
			ID: "tt0206512", Name: "SpongeBob SquarePants (سبونج بوب)", Type: "series", Year: "1999–",
			Poster: "https://images.metahub.space/poster/small/tt0206512/img",
			Backdrop: "https://images.metahub.space/background/medium/tt0206512/img",
			ImdbRating: "8.2", Description: "SpongeBob's nautical nonsense in Bikini Bottom with instant Arabic dub archive episodes.",
			Genres: []string{"Animation", "Comedy"},
		},
		{
			ID: "tt1942683", Name: "The Amazing World of Gumball (عالم غامبول المدهش)", Type: "series", Year: "2011–2019",
			Poster: "https://images.metahub.space/poster/small/tt1942683/img",
			Backdrop: "https://images.metahub.space/background/medium/tt1942683/img",
			ImdbRating: "8.4", Description: "Gumball and Darwin's misadventures in Elmore with Official Cartoon Network Arabic dub.",
			Genres: []string{"Animation", "Comedy"},
		},
		{
			ID: "tt1710308", Name: "Regular Show (العرض العادي)", Type: "series", Year: "2010–2017",
			Poster: "https://images.metahub.space/poster/small/tt1710308/img",
			Backdrop: "https://images.metahub.space/background/medium/tt1710308/img",
			ImdbRating: "8.6", Description: "Mordecai and Rigby turn mundane park chores into surreal cosmic disasters.",
			Genres: []string{"Animation", "Comedy"},
		},
		{
			ID: "tt4938232", Name: "Regular Show: The Movie (العرض العادي: الفيلم)", Type: "movie", Year: "2015",
			Poster: "https://images.metahub.space/poster/small/tt4938232/img",
			Backdrop: "https://images.metahub.space/background/medium/tt4938232/img",
			ImdbRating: "7.4", Description: "Full 65-minute feature film in original official Cartoon Network Arabic dub.",
			Genres: []string{"Animation", "Adventure"},
		},
		{
			ID: "tt1305826", Name: "Adventure Time (وقت المغامرة)", Type: "series", Year: "2010–2018",
			Poster: "https://images.metahub.space/poster/small/tt1305826/img",
			Backdrop: "https://images.metahub.space/background/medium/tt1305826/img",
			ImdbRating: "8.6", Description: "Finn the Human and Jake the Dog traverse the post-apocalyptic Land of Ooo.",
			Genres: []string{"Animation", "Fantasy"},
		},
		{
			ID: "tt0220880", Name: "Courage the Cowardly Dog (كوردج الجبان)", Type: "series", Year: "1999–2002",
			Poster: "https://images.metahub.space/poster/small/tt0220880/img",
			Backdrop: "https://images.metahub.space/background/medium/tt0220880/img",
			ImdbRating: "8.3", Description: "Classic Cartoon Network Arabic dub episodes from Nowhere.",
			Genres: []string{"Animation", "Mystery"},
		},
		{
			ID: "tt1178180", Name: "The Marvelous Misadventures of Flapjack (فلاب جاك)", Type: "series", Year: "2008–2010",
			Poster: "https://images.metahub.space/poster/small/tt1178180/img",
			Backdrop: "https://images.metahub.space/background/medium/tt1178180/img",
			ImdbRating: "7.8", Description: "Flapjack, Captain K'nuckles, and Bubbie search for Candied Island in 1080p.",
			Genres: []string{"Animation", "Comedy"},
		},
		{
			ID: "tt12158994", Name: "Tom and Jerry (1940) Classic Collection", Type: "series", Year: "1940–1968",
			Poster: "https://images.metahub.space/poster/small/tt12158994/img",
			Backdrop: "https://images.metahub.space/background/medium/tt12158994/img",
			ImdbRating: "9.0", Description: "Complete Golden Collection shorts in remastered 1080p BluRay.",
			Genres: []string{"Animation", "Classic"},
		},
	}
}

func pcFetchJSON(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	resp, err := pcHttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

func parseCinemetaList(data []byte, fallbackType string) []CinemaItem {
	return parseCinemetaListFiltered(data, fallbackType, false)
}

func parseCinemetaListFiltered(data []byte, fallbackType string, requireRealPoster bool) []CinemaItem {
	var raw struct {
		Metas []struct {
			ID          string      `json:"id"`
			ImdbID      string      `json:"imdb_id"`
			Name        string      `json:"name"`
			Type        string      `json:"type"`
			ReleaseInfo interface{} `json:"releaseInfo"`
			Year        interface{} `json:"year"`
			Poster      string      `json:"poster"`
			Background  string      `json:"background"`
			Logo        string      `json:"logo"`
			ImdbRating  interface{} `json:"imdbRating"`
			Description string      `json:"description"`
			Genres      []string    `json:"genres"`
		} `json:"metas"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	out := make([]CinemaItem, 0, len(raw.Metas))
	for _, m := range raw.Metas {
		id := m.ID
		if id == "" {
			id = m.ImdbID
		}
		if id == "" || strings.TrimSpace(m.Name) == "" {
			continue
		}
		if requireRealPoster && strings.TrimSpace(m.Poster) == "" {
			continue
		}
		itemType := m.Type
		if itemType == "" || itemType == "anime" {
			itemType = fallbackType
		}
		year := ""
		if m.ReleaseInfo != nil {
			year = fmt.Sprintf("%v", m.ReleaseInfo)
		} else if m.Year != nil {
			year = fmt.Sprintf("%v", m.Year)
		}
		poster := strings.TrimSpace(m.Poster)
		if poster == "" {
			if strings.HasPrefix(id, "kitsu:") {
				poster = fmt.Sprintf("https://media.kitsu.app/anime/poster_images/%s/medium.jpg", strings.TrimPrefix(id, "kitsu:"))
			} else {
				poster = fmt.Sprintf("https://images.metahub.space/poster/small/%s/img", id)
			}
		}
		bg := m.Background
		if bg == "" && strings.HasPrefix(id, "tt") {
			bg = fmt.Sprintf("https://images.metahub.space/background/large/%s/img", id)
		} else if strings.Contains(bg, "images.metahub.space/background/medium/") {
			bg = strings.Replace(bg, "/background/medium/", "/background/large/", 1)
		}
		logo := m.Logo
		if logo == "" && strings.HasPrefix(id, "tt") {
			logo = fmt.Sprintf("https://images.metahub.space/logo/medium/%s/img", id)
		}
		rating := ""
		if m.ImdbRating != nil {
			rStr := strings.TrimSpace(fmt.Sprintf("%v", m.ImdbRating))
			if rStr != "" && rStr != "<nil>" && rStr != "0" && rStr != "N/A" {
				rating = rStr
			}
		}
		out = append(out, CinemaItem{
			ID: id, Name: m.Name, Type: itemType, Year: year,
			Poster: poster, Backdrop: bg, Logo: logo, ImdbRating: rating,
			Description: m.Description, Genres: m.Genres,
		})
	}
	return out
}

func (e *Engine) handlePcCatalog(w http.ResponseWriter, r *http.Request) {
	cat := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("category")))
	genre := strings.TrimSpace(r.URL.Query().Get("genre"))
	if cat == "" {
		cat = "home"
	}
	if genre == "" {
		genre = "Popular"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 9*time.Second)
	defer cancel()

	if cat == "home" {
		var movies, series []CinemaItem
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if b, err := pcFetchJSON(ctx, "https://v3-cinemeta.strem.io/catalog/movie/top.json"); err == nil {
				movies = parseCinemetaList(b, "movie")
				if len(movies) > 28 {
					movies = movies[:28]
				}
			}
		}()
		go func() {
			defer wg.Done()
			if b, err := pcFetchJSON(ctx, "https://v3-cinemeta.strem.io/catalog/series/top.json"); err == nil {
				series = parseCinemetaList(b, "series")
				if len(series) > 28 {
					series = series[:28]
				}
			}
		}()
		wg.Wait()
		writeJSON(w, 200, map[string]interface{}{
			"movies":   movies,
			"series":   series,
			"cartoons": curatedArabicCartoons(),
		})
		return
	}

	if cat == "cartoons" {
		writeJSON(w, 200, map[string]interface{}{"items": curatedArabicCartoons()})
		return
	}

	if cat == "anime" {
		ep := "https://anime-kitsu.strem.fun/catalog/anime/kitsu-anime-list.json"
		if genre != "" && genre != "Popular" && genre != "Top Rated" {
			g := genre
			if g == "Action & Sci-Fi" {
				g = "Action"
			}
			ep = fmt.Sprintf("https://anime-kitsu.strem.fun/catalog/anime/kitsu-anime-list/genre=%s.json", url.QueryEscape(g))
		}
		var items []CinemaItem
		if b, err := pcFetchJSON(ctx, ep); err == nil {
			items = parseCinemetaList(b, "series")
		}
		writeJSON(w, 200, map[string]interface{}{"items": items})
		return
	}

	fetchOne := func(t string) []CinemaItem {
		var u string
		switch genre {
		case "Top Rated":
			u = fmt.Sprintf("https://v3-cinemeta.strem.io/catalog/%s/imdbRating.json", t)
		case "Action & Sci-Fi":
			u = fmt.Sprintf("https://v3-cinemeta.strem.io/catalog/%s/top/genre=Action.json", t)
		case "Thriller", "Comedy", "Drama":
			u = fmt.Sprintf("https://v3-cinemeta.strem.io/catalog/%s/top/genre=%s.json", t, url.QueryEscape(genre))
		default:
			u = fmt.Sprintf("https://v3-cinemeta.strem.io/catalog/%s/top.json", t)
		}
		b, err := pcFetchJSON(ctx, u)
		if err != nil {
			return nil
		}
		return parseCinemetaList(b, t)
	}

	if cat == "all" {
		var m, s []CinemaItem
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); m = fetchOne("movie") }()
		go func() { defer wg.Done(); s = fetchOne("series") }()
		wg.Wait()
		var merged []CinemaItem
		maxL := len(m)
		if len(s) > maxL {
			maxL = len(s)
		}
		for i := 0; i < maxL && len(merged) < 60; i++ {
			if i < len(m) {
				merged = append(merged, m[i])
			}
			if i < len(s) {
				merged = append(merged, s[i])
			}
		}
		writeJSON(w, 200, map[string]interface{}{"items": merged})
		return
	}

	t := "movie"
	if cat == "series" {
		t = "series"
	}
	items := fetchOne(t)
	if len(items) > 60 {
		items = items[:60]
	}
	writeJSON(w, 200, map[string]interface{}{"items": items})
}

func normalizeSearchKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevSpace = false
		} else if !prevSpace {
			b.WriteByte(' ')
			prevSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func scoreSearchCandidate(item CinemaItem, qNorm string, rankInCat int, isAnime bool) int {
	nameNorm := normalizeSearchKey(item.Name)
	score := 0

	// 1. Preserve provider popularity rank strongly across all 3 categories (Movies, Series, Anime)
	// Index 0 in Movies, Index 0 in Series, and Index 0 in Anime all compete evenly at the top!
	if rankInCat < 20 {
		score += (20 - rankInCat) * 32
	}

	// 2. Title match quality
	if nameNorm == qNorm {
		score += 520
	} else if strings.HasPrefix(nameNorm, qNorm+" ") || strings.HasPrefix(nameNorm, qNorm+"s ") {
		score += 320
	} else if strings.HasPrefix(nameNorm, qNorm) {
		score += 240
	} else if strings.Contains(" "+nameNorm+" ", " "+qNorm+" ") {
		score += 130
	} else if strings.Contains(nameNorm, qNorm) {
		score += 40
	} else {
		score -= 180
	}

	// 3. Real IMDb / Kitsu rating rewards iconic mainstream titles over mockbusters
	if item.ImdbRating != "" {
		if rv, err := strconv.ParseFloat(item.ImdbRating, 64); err == nil {
			if rv >= 8.0 {
				score += 220
			} else if rv >= 7.0 {
				score += 130
			} else if rv >= 6.0 {
				score += 50
			} else if rv < 5.0 {
				score -= 120
			}
		}
	}

	// 4. Extra boost for top-ranked Series/Anime so iconic shows ("JoJo", "86", "Breaking Bad") never sink under random obscure movies
	if rankInCat <= 2 && (item.Type == "series" || isAnime) {
		score += 110
	}
	if item.Year != "" {
		score += 25
	}
	return score
}

func (e *Engine) handlePcSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	filterType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type"))) // "", "all", "movie", "series", "anime"
	if len(q) < 2 {
		writeJSON(w, 200, map[string]interface{}{"items": []CinemaItem{}})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	var m, s, a []CinemaItem
	var wg sync.WaitGroup
	wg.Add(3)
	enc := url.PathEscape(q)
	go func() {
		defer wg.Done()
		if filterType == "series" || filterType == "anime" {
			return
		}
		if b, err := pcFetchJSON(ctx, "https://v3-cinemeta.strem.io/catalog/movie/top/search="+enc+".json"); err == nil {
			m = parseCinemetaListFiltered(b, "movie", true)
		}
	}()
	go func() {
		defer wg.Done()
		if filterType == "movie" || filterType == "anime" {
			return
		}
		if b, err := pcFetchJSON(ctx, "https://v3-cinemeta.strem.io/catalog/series/top/search="+enc+".json"); err == nil {
			s = parseCinemetaListFiltered(b, "series", true)
		}
	}()
	go func() {
		defer wg.Done()
		if filterType == "movie" {
			return
		}
		if b, err := pcFetchJSON(ctx, "https://anime-kitsu.strem.fun/catalog/anime/kitsu-anime-list/search="+enc+".json"); err == nil {
			a = parseCinemetaListFiltered(b, "series", true)
		}
	}()
	wg.Wait()

	type scoredHit struct {
		item  CinemaItem
		score int
	}

	qNorm := normalizeSearchKey(q)
	ql := strings.ToLower(q)
	var hits []scoredHit

	if filterType == "" || filterType == "all" || filterType == "series" {
		for _, c := range curatedArabicCartoons() {
			if strings.Contains(strings.ToLower(c.Name), ql) {
				hits = append(hits, scoredHit{item: c, score: scoreSearchCandidate(c, qNorm, 0, false) + 200})
			}
		}
	}
	for i, item := range m {
		if i >= 16 {
			break
		}
		hits = append(hits, scoredHit{item: item, score: scoreSearchCandidate(item, qNorm, i, false)})
	}
	for i, item := range s {
		if i >= 16 {
			break
		}
		hits = append(hits, scoredHit{item: item, score: scoreSearchCandidate(item, qNorm, i, false)})
	}
	for i, item := range a {
		if i >= 14 {
			break
		}
		hits = append(hits, scoredHit{item: item, score: scoreSearchCandidate(item, qNorm, i, true)})
	}

	sort.SliceStable(hits, func(i, j int) bool {
		return hits[i].score > hits[j].score
	})

	seenID := make(map[string]bool)
	seenTitleType := make(map[string]bool)
	var out []CinemaItem
	for _, h := range hits {
		if seenID[h.item.ID] {
			continue
		}
		ttKey := normalizeSearchKey(h.item.Name) + "|" + h.item.Type + "|" + h.item.Year
		if seenTitleType[ttKey] {
			continue
		}
		seenID[h.item.ID] = true
		seenTitleType[ttKey] = true
		out = append(out, h.item)
		if len(out) >= 36 {
			break
		}
	}
	writeJSON(w, 200, map[string]interface{}{"items": out})
}

func (e *Engine) handlePcMeta(w http.ResponseWriter, r *http.Request) {
	mtype := strings.TrimSpace(r.URL.Query().Get("type"))
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		http.Error(w, "Missing id", 400)
		return
	}
	if mtype == "" {
		mtype = "movie"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	ep := fmt.Sprintf("https://v3-cinemeta.strem.io/meta/%s/%s.json", mtype, url.PathEscape(id))
	if strings.HasPrefix(id, "kitsu:") {
		ep = fmt.Sprintf("https://anime-kitsu.strem.fun/meta/anime/%s.json", url.PathEscape(id))
	}
	b, err := pcFetchJSON(ctx, ep)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	var raw struct {
		Meta struct {
			ID          string      `json:"id"`
			Name        string      `json:"name"`
			Type        string      `json:"type"`
			ReleaseInfo interface{} `json:"releaseInfo"`
			Year        interface{} `json:"year"`
			Poster      string      `json:"poster"`
			Background  string      `json:"background"`
			Logo        string      `json:"logo"`
			ImdbRating  interface{} `json:"imdbRating"`
			Description string      `json:"description"`
			Genres      []string    `json:"genres"`
			Cast        []string    `json:"cast"`
			Runtime     string      `json:"runtime"`
			Videos      []struct {
				ID        string `json:"id"`
				Season    int    `json:"season"`
				Episode   int    `json:"episode"`
				Number    int    `json:"number"`
				Title     string `json:"title"`
				Name      string `json:"name"`
				Thumbnail string `json:"thumbnail"`
				Overview  string `json:"overview"`
				Released  string `json:"released"`
			} `json:"videos"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	m := raw.Meta
	year := ""
	if m.ReleaseInfo != nil {
		year = fmt.Sprintf("%v", m.ReleaseInfo)
	} else if m.Year != nil {
		year = fmt.Sprintf("%v", m.Year)
	}
	rating := ""
	if m.ImdbRating != nil {
		rating = fmt.Sprintf("%v", m.ImdbRating)
	}
	eps := make([]CinemaEpisode, 0, len(m.Videos))
	for _, v := range m.Videos {
		s := v.Season
		if s == 0 && strings.HasPrefix(id, "kitsu:") {
			s = 1
		}
		epNum := v.Episode
		if epNum == 0 {
			epNum = v.Number
		}
		title := v.Title
		if title == "" {
			title = v.Name
		}
		if title == "" {
			title = fmt.Sprintf("Episode %d", epNum)
		}
		rel := v.Released
		if len(rel) >= 10 {
			rel = rel[:10]
		}
		eps = append(eps, CinemaEpisode{
			ID: v.ID, Season: s, Episode: epNum, Title: title,
			Thumbnail: v.Thumbnail, Overview: v.Overview, Released: rel,
		})
	}
	bg := m.Background
	if bg == "" && strings.HasPrefix(id, "tt") {
		bg = fmt.Sprintf("https://images.metahub.space/background/large/%s/img", id)
	} else if strings.Contains(bg, "images.metahub.space/background/medium/") {
		bg = strings.Replace(bg, "/background/medium/", "/background/large/", 1)
	}
	logo := m.Logo
	if logo == "" && strings.HasPrefix(id, "tt") {
		logo = fmt.Sprintf("https://images.metahub.space/logo/medium/%s/img", id)
	}
	writeJSON(w, 200, CinemaDeepMeta{
		ID: m.ID, Name: m.Name, Type: mtype, Year: year,
		Poster: m.Poster, Background: bg, Logo: logo, ImdbRating: rating,
		Description: m.Description, Genres: m.Genres, Cast: m.Cast,
		Runtime: m.Runtime, Videos: eps,
	})
}

var (
	reSeeders = regexp.MustCompile(`👤\s*(\d+)`)
	reSize    = regexp.MustCompile(`(?i)💾\s*([\d.]+\s*[GMKT]B)`)
)

func scoreStreamForProfile(s CinemaStream, profile string) int {
	if s.AudioURL != "" || strings.Contains(s.URL, "archive.org") {
		return 100000
	}
	if s.IsDirect && s.Seeders == 0 {
		return 95000
	}
	if s.Seeders <= 0 {
		return -10000
	}
	comb := strings.ToLower(s.Title + " " + s.Name)
	if strings.Contains(comb, "camrip") || strings.Contains(comb, "telesync") || strings.Contains(comb, "hdcam") {
		return -50000
	}

	gb := 0.0
	sz := strings.ToLower(s.SizeText)
	if strings.Contains(sz, "gb") {
		gb, _ = strconv.ParseFloat(strings.TrimSpace(strings.ReplaceAll(sz, "gb", "")), 64)
	} else if strings.Contains(sz, "mb") {
		mb, _ := strconv.ParseFloat(strings.TrimSpace(strings.ReplaceAll(sz, "mb", "")), 64)
		gb = mb / 1024.0
	}

	isRemux := strings.Contains(comb, "remux")
	isWeb := strings.Contains(comb, "web-dl") || strings.Contains(comb, "webrip")

	// Penalize single-language non-English dubs (Russian/Italian/Hindi/French-only) unless English/Multi/Dual is also present
	hasEngOrMulti := strings.Contains(comb, "🇬🇧") || strings.Contains(comb, "🇺🇸") ||
		strings.Contains(comb, "multi") || strings.Contains(comb, "dual") ||
		strings.Contains(comb, "eng") || strings.Contains(comb, "english")
	hasForeignTag := strings.Contains(comb, "🇷🇺") || strings.Contains(comb, "🇮🇹") ||
		strings.Contains(comb, "🇮🇳") || strings.Contains(comb, "🇫🇷") ||
		strings.Contains(comb, "🇪🇸") || strings.Contains(comb, "🇩🇪") ||
		strings.Contains(comb, "🇵🇱") || strings.Contains(comb, "🇧🇷") ||
		strings.Contains(comb, "rus") || strings.Contains(comb, "vostfr") ||
		strings.Contains(comb, "french") || strings.Contains(comb, "italian") ||
		strings.Contains(comb, "hindi") || strings.Contains(comb, "latino") ||
		strings.Contains(comb, "dublado") || strings.Contains(comb, "sub ita")
	langScore := 0
	if hasForeignTag && !hasEngOrMulti {
		langScore = -3200
	} else if hasEngOrMulti {
		langScore = 350
	}

	baseQ := 4000
	if profile == "fiber4k" {
		switch {
		case strings.HasPrefix(s.Quality, "4K"):
			baseQ = 10500
		case s.Quality == "1080p":
			baseQ = 9000
		case s.Quality == "720p":
			baseQ = 6000
		}
	} else {
		// BALANCED / MEH-WIFI PROFILE:
		// Prioritizes fast-starting 1080p WEB-DL (0.8-3.8 GB) with strong seeders so playback NEVER stalls!
		switch {
		case s.Quality == "1080p":
			baseQ = 10200
		case s.Quality == "720p":
			baseQ = 7600
		case strings.HasPrefix(s.Quality, "4K"):
			if gb > 0 && gb <= 5.5 && s.Seeders >= 30 {
				baseQ = 8600
			} else {
				baseQ = 3000
			}
		}
	}

	seedScore := -3000
	switch {
	case s.Seeders >= 120:
		seedScore = 4000
	case s.Seeders >= 50:
		seedScore = 3100
	case s.Seeders >= 20:
		seedScore = 2200
	case s.Seeders >= 8:
		seedScore = 1100
	case s.Seeders >= 3:
		seedScore = 300
	}

	sizeScore := 0
	if profile == "balanced" {
		switch {
		case gb >= 0.4 && gb <= 3.8:
			sizeScore = 3200
		case gb > 3.8 && gb <= 6.5:
			sizeScore = 600
		case gb > 6.5 && gb <= 12.0:
			sizeScore = -6000
		case gb > 12.0:
			sizeScore = -22000
		}
	} else {
		if gb > 25.0 {
			sizeScore = -12000
		}
	}

	bonus := 0
	if isWeb {
		bonus += 1200
	}
	if isRemux && profile == "balanced" {
		bonus -= 12000
	}
	return baseQ + seedScore + sizeScore + bonus + langScore
}

func resolveCuratedArabicStreams(id string, season, episode int, mtype string) []CinemaStream {
	var out []CinemaStream
	if strings.Contains(id, "tt3718778") {
		cnMovie := "https://archive.org/download/20261003_20261003_1042/%D8%AE%D9%84%D9%81%20%D8%AD%D8%A7%D8%A6%D8%B7%20%D8%A7%D9%84%D8%AD%D8%AF%D9%8A%D9%82%D8%A9%20%D9%83%D8%B1%D8%AA%D9%88%D9%86%20%D9%86%D8%AA%D9%88%D8%B1%D9%83%20%D8%A8%D8%A7%D9%84%D8%B9%D8%B1%D8%A8%D9%8A%D8%A9%20%D8%A3%D9%81%D9%84%D8%A7%D9%85.mp4"
		out = append(out, CinemaStream{
			Name: "Cartoon Network Arabic", Title: "خلف حائط الحديقة — الفيلم الكامل (CN Arabic Dub)",
			URL: cnMovie, Quality: "1080p", Seeders: 99999, SizeText: "664 MB",
			ReleaseTitle: "Over.the.Garden.Wall.CN.Arabic.78min.mp4",
			CodecTags: "Direct Instant HTTPS • Official CN Arabic Dub", HasArabicHint: true, IsDirect: true,
		})
	}
	if strings.Contains(id, "tt0206512") && season >= 1 && season <= 12 {
		num := episode
		if season == 2 && num > 38 {
			num = 38
		} else if season != 2 && num > 25 {
			num = (num + 1) / 2
		}
		if num < 1 {
			num = 1
		}
		out = append(out, CinemaStream{
			Name: "Archive.org Arabic Dub", Title: fmt.Sprintf("سبونج بوب — S%02dE%02d (Arabic Dub)", season, episode),
			URL: fmt.Sprintf("https://archive.org/download/Spongebob.Squarepants.S%02d/%02d.mp4", season, num),
			Quality: "1080p", Seeders: 99999, SizeText: "200 MB",
			ReleaseTitle: fmt.Sprintf("SpongeBob.Arabic.Dub.S%02dE%02d.mp4", season, num),
			CodecTags: "Direct Instant HTTPS • Arabic Audio", HasArabicHint: true, IsDirect: true,
		})
	}
	if strings.Contains(id, "tt1942683") {
		ep := episode
		if season > 1 {
			ep = (season-1)*36 + episode
		}
		if ep < 1 {
			ep = 1
		}
		if ep > 48 {
			ep = 48
		}
		fname := url.PathEscape(fmt.Sprintf("عالم غامبول المدهش مدبلج الحلقة %02d.mp4", ep))
		out = append(out, CinemaStream{
			Name: "Cartoon Network Arabic", Title: fmt.Sprintf("عالم غامبول المدهش — الحلقة %02d (CN Arabic)", ep),
			URL: "https://archive.org/download/03_20251029/" + fname, Quality: "1080p", Seeders: 99999, SizeText: "150 MB",
			ReleaseTitle: fmt.Sprintf("Gumball.CN.Arabic.Ep%02d.mp4", ep),
			CodecTags: "Direct Instant HTTPS • Official CN Arabic Dub", HasArabicHint: true, IsDirect: true,
		})
	}
	if strings.Contains(id, "tt4938232") || (strings.Contains(id, "tt1710308") && mtype == "movie") {
		out = append(out, CinemaStream{
			Name: "Cartoon Network Arabic", Title: "العرض العادي: الفيلم الكامل (CN Arabic Dub)",
			URL: "https://archive.org/download/20261017_20261017_0821/%D8%A7%D9%84%D8%B9%D8%B1%D8%B6%20%D8%A7%D9%84%D8%B9%D8%A7%D8%AF%D9%8A%20%D8%A7%D9%84%D9%81%D9%8A%D9%84%D9%85%20%D9%83%D8%B1%D8%AA%D9%88%D9%86%20%D9%86%D8%AA%D9%88%D8%B1%D9%83%20%D8%A8%D8%A7%D9%84%D8%B9%D8%B1%D8%A8%D9%8A%D8%A9%20%D8%A3%D9%81%D9%84%D8%A7%D9%85.mp4",
			Quality: "1080p", Seeders: 99999, SizeText: "548 MB",
			ReleaseTitle: "Regular.Show.The.Movie.CN.Arabic.65min.mp4",
			CodecTags: "Direct Instant HTTPS • Official CN Arabic Dub", HasArabicHint: true, IsDirect: true,
		})
	}
	if strings.Contains(id, "tt1305826") && season == 1 && episode == 1 {
		out = append(out, CinemaStream{
			Name: "Cartoon Network Arabic", Title: "وقت المغامرة — S01E01 (CN Arabic Dub)",
			URL: "https://archive.org/download/adventure-time-s-01e-01-dub-star-dima-1/Adventure_Time_S01e01_dub_%5BStarDima%5D.mp4",
			Quality: "1080p", Seeders: 99999, SizeText: "272 MB",
			ReleaseTitle: "Adventure.Time.S01E01.CN.Arabic.mp4",
			CodecTags: "Direct Instant HTTPS • Arabic Dub", HasArabicHint: true, IsDirect: true,
		})
	}
	if strings.Contains(id, "tt0220880") {
		ep := episode
		if ep < 1 {
			ep = 1
		}
		if ep > 32 {
			ep = 32
		}
		out = append(out, CinemaStream{
			Name: "Cartoon Network Arabic", Title: fmt.Sprintf("كوردج الجبان — الحلقة %02d (CN Arabic)", ep),
			URL: fmt.Sprintf("https://archive.org/download/courage_aljban_25/courage_aljban_%02d.mp4", ep),
			Quality: "1080p", Seeders: 99999, SizeText: "72 MB",
			ReleaseTitle: fmt.Sprintf("Courage.Cowardly.Dog.Ep%02d.CN.Arabic.mp4", ep),
			CodecTags: "Direct Instant HTTPS • Official CN Arabic Dub", HasArabicHint: true, IsDirect: true,
		})
	}
	return out
}

func (e *Engine) handlePcStreams(w http.ResponseWriter, r *http.Request) {
	mtype := strings.TrimSpace(r.URL.Query().Get("type"))
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	season, _ := strconv.Atoi(r.URL.Query().Get("season"))
	episode, _ := strconv.Atoi(r.URL.Query().Get("episode"))
	if season <= 0 {
		season = 1
	}
	if episode <= 0 {
		episode = 1
	}
	profile := strings.TrimSpace(r.URL.Query().Get("profile"))
	if profile == "" {
		profile = pcWifiProfile
	}

	streamType := mtype
	if streamType == "anime" {
		streamType = "series"
	}
	queryID := id
	if streamType == "series" {
		if strings.HasPrefix(id, "kitsu:") {
			if strings.Count(id, ":") < 2 {
				queryID = fmt.Sprintf("%s:%d", id, episode)
			}
		} else if !strings.Contains(id, ":") {
			queryID = fmt.Sprintf("%s:%d:%d", id, season, episode)
		}
	}

	endpoints := []string{
		fmt.Sprintf("https://torrentio.strem.fun/sort=qualitysize|qualityfilter=scr,cam/stream/%s/%s.json", streamType, queryID),
		fmt.Sprintf("https://comet.elfhosted.com/stream/%s/%s.json", streamType, queryID),
		fmt.Sprintf("https://mediafusion.elfhosted.com/stream/%s/%s.json", streamType, queryID),
	}

	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()

	var mu sync.Mutex
	streams := resolveCuratedArabicStreams(id, season, episode, streamType)

	var wg sync.WaitGroup
	for _, ep := range endpoints {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			b, err := pcFetchJSON(ctx, u)
			if err != nil {
				return
			}
			var raw struct {
				Streams []struct {
					Name     string   `json:"name"`
					Title    string   `json:"title"`
					InfoHash string   `json:"infoHash"`
					FileIdx  *int     `json:"fileIdx"`
					URL      string   `json:"url"`
					Sources  []string `json:"sources"`
				} `json:"streams"`
			}
			if json.Unmarshal(b, &raw) != nil {
				return
			}
			var local []CinemaStream
			for _, st := range raw.Streams {
				if st.InfoHash == "" && st.URL == "" {
					continue
				}
				seeders := 0
				if m := reSeeders.FindStringSubmatch(st.Title); len(m) > 1 {
					seeders, _ = strconv.Atoi(m[1])
				}
				sizeText := ""
				if m := reSize.FindStringSubmatch(st.Title); len(m) > 1 {
					sizeText = m[1]
				}
				upper := strings.ToUpper(st.Name + " " + st.Title)
				qual := "HD"
				switch {
				case strings.Contains(upper, "4K") || strings.Contains(upper, "2160P") || strings.Contains(upper, "UHD"):
					qual = "4K HDR"
				case strings.Contains(upper, "1080P") || strings.Contains(upper, "FHD"):
					qual = "1080p"
				case strings.Contains(upper, "720P"):
					qual = "720p"
				}
				firstLine := strings.TrimSpace(strings.Split(st.Title, "\n")[0])
				if len(firstLine) > 84 {
					firstLine = firstLine[:84]
				}
				var tags []string
				if strings.Contains(upper, "WEB-DL") || strings.Contains(upper, "WEBRIP") {
					tags = append(tags, "WEB-DL")
				} else if strings.Contains(upper, "BLURAY") {
					tags = append(tags, "BluRay")
				}
				if strings.Contains(upper, "HEVC") || strings.Contains(upper, "X265") || strings.Contains(upper, "H265") {
					tags = append(tags, "HEVC")
				} else if strings.Contains(upper, "X264") || strings.Contains(upper, "H264") {
					tags = append(tags, "H.264")
				}
				hasAr := strings.Contains(upper, "ARA") || strings.Contains(upper, "ARABIC") || strings.Contains(upper, "MULTI")
				item := CinemaStream{
					Name: st.Name, Title: st.Title, InfoHash: strings.ToLower(st.InfoHash),
					FileIdx: st.FileIdx, URL: st.URL, Sources: st.Sources,
					Quality: qual, Seeders: seeders, SizeText: sizeText,
					ReleaseTitle: firstLine, CodecTags: strings.Join(tags, " • "),
					HasArabicHint: hasAr, IsDirect: st.URL != "",
				}
				local = append(local, item)
			}
			mu.Lock()
			streams = append(streams, local...)
			mu.Unlock()
		}(ep)
	}
	wg.Wait()

	seen := make(map[string]bool)
	var unique []CinemaStream
	for _, s := range streams {
		k := s.URL
		if k == "" {
			idx := -1
			if s.FileIdx != nil {
				idx = *s.FileIdx
			}
			k = fmt.Sprintf("%s:%d", s.InfoHash, idx)
		}
		if seen[k] {
			continue
		}
		seen[k] = true
		s.Score = scoreStreamForProfile(s, profile)
		unique = append(unique, s)
	}
	sort.Slice(unique, func(i, j int) bool {
		return unique[i].Score > unique[j].Score
	})
	if len(unique) > 40 {
		unique = unique[:40]
	}

	writeJSON(w, 200, map[string]interface{}{
		"profile": profile,
		"streams": unique,
	})
}

func (e *Engine) handlePcSubtitles(w http.ResponseWriter, r *http.Request) {
	mtype := strings.TrimSpace(r.URL.Query().Get("type"))
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	season := r.URL.Query().Get("season")
	episode := r.URL.Query().Get("episode")
	if mtype == "anime" {
		mtype = "series"
	}
	qid := id
	if mtype == "series" && !strings.Contains(id, ":") && season != "" && episode != "" {
		qid = fmt.Sprintf("%s:%s:%s", id, season, episode)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()

	ep := fmt.Sprintf("https://opensubtitles-v3.strem.io/subtitles/%s/%s.json", mtype, qid)
	b, err := pcFetchJSON(ctx, ep)
	if err != nil {
		writeJSON(w, 200, map[string]interface{}{"subtitles": []SubtitleTrack{}})
		return
	}
	var raw struct {
		Subtitles []struct {
			ID   string `json:"id"`
			Lang string `json:"lang"`
			URL  string `json:"url"`
		} `json:"subtitles"`
	}
	_ = json.Unmarshal(b, &raw)

	var out []SubtitleTrack
	for _, s := range raw.Subtitles {
		l := strings.ToLower(s.Lang)
		label := strings.ToUpper(s.Lang)
		priority := false
		switch {
		case strings.HasPrefix(l, "ar"):
			label = "Arabic (العربية)"
			priority = true
		case strings.HasPrefix(l, "en"):
			label = "English"
			priority = true
		case strings.HasPrefix(l, "fr"):
			label = "French (Français)"
			priority = true
		}
		if !priority && len(out) >= 16 {
			continue
		}
		out = append(out, SubtitleTrack{ID: s.ID, LangCode: l, LangLabel: label, URL: s.URL})
	}
	// Put Arabic & English at the top
	sort.SliceStable(out, func(i, j int) bool {
		rank := func(c string) int {
			if strings.HasPrefix(c, "ar") {
				return 0
			}
			if strings.HasPrefix(c, "en") {
				return 1
			}
			if strings.HasPrefix(c, "fr") {
				return 2
			}
			return 3
		}
		return rank(out[i].LangCode) < rank(out[j].LangCode)
	})
	if len(out) > 20 {
		out = out[:20]
	}
	writeJSON(w, 200, map[string]interface{}{"subtitles": out})
}

func (e *Engine) handlePcSubtitleVtt(w http.ResponseWriter, r *http.Request) {
	rawURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if rawURL == "" {
		http.Error(w, "Missing url", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	b, err := pcFetchJSON(ctx, rawURL)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	text := string(b)
	if !utf8.Valid(b) {
		// Minimal fallback decode for legacy Windows-1256 / Latin-1 bytes
		text = strings.ToValidUTF8(text, "")
	}
	text = strings.TrimPrefix(text, "\xef\xbb\xbf")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if !strings.HasPrefix(strings.TrimSpace(text), "WEBVTT") {
		reTime := regexp.MustCompile(`(\d{2}:\d{2}:\d{2}),(\d{3})`)
		text = "WEBVTT\n\n" + reTime.ReplaceAllString(text, "$1.$2")
	}
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_, _ = w.Write([]byte(text))
}

// ==============================================================================
// FFMPEG AUDIO TRACK PROBE + SMART STREAM PIPE (AUTO-ENGLISH + LANGUAGE SWITCHER)
// ==============================================================================
type ProbedMediaInfo struct {
	Tracks      []AudioTrack
	DurationSec float64
}

var (
	pcAudioCacheMu sync.Mutex
	pcAudioCache   = make(map[string]ProbedMediaInfo)
	reAudioStream  = regexp.MustCompile(`Stream\s+#0:(\d+)(?:\(([^)]+)\))?[^:]*:\s*Audio:\s*([^,\r\n]+)(?:,\s*([^,\r\n]+))?(?:,\s*([^,\r\n]+))?`)
	reMetaTitle    = regexp.MustCompile(`(?i)^\s+title\s*:\s*(.+)$`)
	reMediaDur     = regexp.MustCompile(`Duration:\s*(\d+):(\d+):(\d+(?:\.\d+)?)`)
)

func langCodeToLabel(code string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	switch c {
	case "eng", "en", "en-us", "en-gb":
		return "English"
	case "jpn", "ja", "jp":
		return "Japanese"
	case "ara", "ar":
		return "Arabic"
	case "fre", "fra", "fr":
		return "French"
	case "spa", "es":
		return "Spanish"
	case "rus", "ru":
		return "Russian"
	case "ita", "it":
		return "Italian"
	case "ger", "deu", "de":
		return "German"
	case "hin", "hi":
		return "Hindi"
	case "por", "pt":
		return "Portuguese"
	case "kor", "ko":
		return "Korean"
	case "chi", "zho", "zh":
		return "Chinese"
	case "tur", "tr":
		return "Turkish"
	case "pol", "pl":
		return "Polish"
	case "ukr", "uk":
		return "Ukrainian"
	case "und", "":
		return "Original / Default"
	default:
		return strings.ToUpper(c)
	}
}

func audioCacheKey(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil {
		if h := u.Query().Get("hash"); h != "" {
			return h + ":" + u.Query().Get("idx")
		}
	}
	return rawURL
}

func probeMediaInfo(ctx context.Context, rawURL string) ProbedMediaInfo {
	key := audioCacheKey(rawURL)
	pcAudioCacheMu.Lock()
	if cached, ok := pcAudioCache[key]; ok && (len(cached.Tracks) > 0 || cached.DurationSec > 0) {
		pcAudioCacheMu.Unlock()
		return cached
	}
	pcAudioCacheMu.Unlock()

	ffmpeg := detectFfmpegBinary()
	if ffmpeg == "" || rawURL == "" {
		return ProbedMediaInfo{}
	}

	probeCtx, cancel := context.WithTimeout(ctx, 4500*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(probeCtx, ffmpeg,
		"-hide_banner",
		"-probesize", "1500000",
		"-analyzeduration", "1500000",
		"-i", rawURL,
	)
	hideCmdWindow(cmd)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	_ = cmd.Run()

	stderrStr := errBuf.String()
	var durSec float64
	if dm := reMediaDur.FindStringSubmatch(stderrStr); dm != nil {
		hh, _ := strconv.ParseFloat(dm[1], 64)
		mm, _ := strconv.ParseFloat(dm[2], 64)
		ss, _ := strconv.ParseFloat(dm[3], 64)
		durSec = hh*3600.0 + mm*60.0 + ss
	}

	lines := strings.Split(stderrStr, "\n")
	var tracks []AudioTrack
	audioSeq := 0
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		m := reAudioStream.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		lang := strings.ToLower(strings.TrimSpace(m[2]))
		if lang == "" {
			lang = "und"
		}
		codec := strings.TrimSpace(m[3])
		if sp := strings.IndexByte(codec, ' '); sp > 0 {
			codec = codec[:sp]
		}
		codec = strings.ToUpper(codec)
		if ch := strings.TrimSpace(m[5]); ch != "" && (strings.Contains(ch, "5.1") || strings.Contains(ch, "7.1") || strings.Contains(ch, "stereo") || strings.Contains(ch, "mono")) {
			codec += " " + ch
		}
		title := ""
		// Peek next few indented metadata lines for track title (e.g. "English Original", "Russian Dub")
		for j := i + 1; j < len(lines) && j <= i+6; j++ {
			next := lines[j]
			if strings.Contains(next, "Stream #0:") {
				break
			}
			if tm := reMetaTitle.FindStringSubmatch(next); tm != nil {
				title = strings.TrimSpace(tm[1])
				break
			}
		}
		tracks = append(tracks, AudioTrack{
			AudioIdx:  audioSeq,
			LangCode:  lang,
			LangLabel: langCodeToLabel(lang),
			Codec:     codec,
			Title:     title,
		})
		audioSeq++
	}

	info := ProbedMediaInfo{
		Tracks:      tracks,
		DurationSec: durSec,
	}
	if len(tracks) > 0 || durSec > 0 {
		pcAudioCacheMu.Lock()
		pcAudioCache[key] = info
		pcAudioCacheMu.Unlock()
	}
	return info
}

func probeAudioTracks(ctx context.Context, rawURL string) []AudioTrack {
	return probeMediaInfo(ctx, rawURL).Tracks
}

func pickPreferredAudioIdx(tracks []AudioTrack, prefLang string) int {
	if len(tracks) == 0 {
		return 0
	}
	pref := strings.ToLower(strings.TrimSpace(prefLang))
	if pref == "" {
		pref = "eng"
	}
	// 1. Prefer exact non-commentary match for requested language (default: English)
	for _, t := range tracks {
		tl := strings.ToLower(t.Title)
		isCommentary := strings.Contains(tl, "commentary") || strings.Contains(tl, "director") || strings.Contains(tl, "descriptive")
		if isCommentary {
			continue
		}
		if t.LangCode == pref || (pref == "eng" && (t.LangCode == "en" || strings.Contains(tl, "english") || strings.Contains(tl, "original"))) {
			return t.AudioIdx
		}
	}
	// 2. Fallback: any match for preferred language
	for _, t := range tracks {
		if t.LangCode == pref || (pref == "eng" && t.LangCode == "en") {
			return t.AudioIdx
		}
	}
	return 0
}

func (e *Engine) handlePcAudioTracks(w http.ResponseWriter, r *http.Request) {
	rawURL := strings.TrimSpace(r.URL.Query().Get("url"))
	pref := strings.TrimSpace(r.URL.Query().Get("pref"))
	if rawURL == "" {
		writeJSON(w, 200, map[string]interface{}{"tracks": []AudioTrack{}, "defaultIdx": 0, "durationSec": 0})
		return
	}
	info := probeMediaInfo(r.Context(), rawURL)
	defIdx := pickPreferredAudioIdx(info.Tracks, pref)
	writeJSON(w, 200, map[string]interface{}{
		"tracks":      info.Tracks,
		"defaultIdx":  defIdx,
		"durationSec": info.DurationSec,
	})
}

func (e *Engine) handlePcFfmpegStream(w http.ResponseWriter, r *http.Request) {
	ffmpeg := detectFfmpegBinary()
	if ffmpeg == "" {
		http.Error(w, "ffmpeg.exe not found next to NOCTURNE.exe", 503)
		return
	}
	rawURL := strings.TrimSpace(r.URL.Query().Get("url"))
	mode := strings.TrimSpace(r.URL.Query().Get("mode")) // "live" or "vod"
	slot := strings.TrimSpace(r.URL.Query().Get("slot"))
	seekSec := strings.TrimSpace(r.URL.Query().Get("ss"))
	aidxParam := strings.TrimSpace(r.URL.Query().Get("aidx"))
	prefLang := strings.TrimSpace(r.URL.Query().Get("lang"))
	if slot == "" {
		slot = "0"
	}
	if rawURL == "" {
		http.Error(w, "Missing url", 400)
		return
	}

	// Kill previous FFmpeg process on this slot cleanly
	pcFfmpegProcsMu.Lock()
	if old, ok := pcFfmpegProcs[slot]; ok && old.Process != nil {
		_ = old.Process.Kill()
	}
	pcFfmpegProcsMu.Unlock()

	var args []string
	if mode == "vod" {
		// Determine which audio track to map:
		// If explicit aidx=N is supplied by the player's language picker, use it;
		// otherwise probe header (cached) and automatically prefer English instead of random foreign track 0!
		chosenAudio := 0
		if aidxParam != "" && aidxParam != "auto" {
			if parsed, err := strconv.Atoi(aidxParam); err == nil && parsed >= 0 {
				chosenAudio = parsed
			}
		} else {
			tracks := probeAudioTracks(r.Context(), rawURL)
			if len(tracks) > 0 {
				chosenAudio = pickPreferredAudioIdx(tracks, prefLang)
			}
		}

		// VOD Torrent/Archive Smart Remux:
		// Zero-CPU Video Copy (-c:v copy) + Stereo AAC Audio conversion (-c:a aac) into browser-native fMP4!
		args = append(args, "-hide_banner", "-loglevel", "error")
		if seekSec != "" && seekSec != "0" {
			args = append(args, "-ss", seekSec)
		}
		args = append(args,
			"-i", rawURL,
			"-map", "0:v:0",
			"-map", fmt.Sprintf("0:a:%d?", chosenAudio),
			"-c:v", "copy",
			"-c:a", "aac",
			"-b:a", "192k",
			"-ac", "2",
			"-f", "mp4",
			"-movflags", "frag_keyframe+empty_moov+default_base_moof",
			"pipe:1",
		)
	} else {
		// LIVE TV Broadcast Pipe (B-Frame Presentation Order Preserved + bwdif Deinterlacer)
		args = []string{
			"-hide_banner", "-loglevel", "error",
			"-user_agent", "TiviMate/4.7.0 (Linux; Android 11; Shield Android TV)",
			"-reconnect", "1", "-reconnect_streamed", "1", "-reconnect_delay_max", "4",
			"-probesize", "1000000", "-analyzeduration", "1000000",
			"-i", rawURL,
			"-vf", "bwdif=mode=0:parity=-1:deint=0",
			"-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency",
			"-pix_fmt", "yuv420p", "-g", "50",
			"-c:a", "aac", "-b:a", "128k", "-ac", "2", "-ar", "44100",
			"-f", "mp4",
			"-movflags", "frag_keyframe+empty_moov+default_base_moof",
			"pipe:1",
		}
	}

	cmd := exec.CommandContext(r.Context(), ffmpeg, args...)
	hideCmdWindow(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Start(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	pcFfmpegProcsMu.Lock()
	pcFfmpegProcs[slot] = cmd
	pcFfmpegProcsMu.Unlock()

	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 32768)
	for {
		n, readErr := stdout.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				break
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr != nil {
			break
		}
	}
	if errBuf.Len() > 0 {
		log.Printf("[FFmpeg:%s] %s", mode, strings.TrimSpace(errBuf.String()))
	}
}

func (e *Engine) handlePcState(w http.ResponseWriter, r *http.Request) {
	pcStateMu.Lock()
	defer pcStateMu.Unlock()
	if pcStateFile == "" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = "."
		}
		dir := filepath.Join(base, "NOCTURNE")
		_ = os.MkdirAll(dir, 0755)
		pcStateFile = filepath.Join(dir, "nocturne_state.json")
	}
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
		if err == nil && len(body) > 2 {
			_ = os.WriteFile(pcStateFile, body, 0644)
			var parsed struct {
				Profile string `json:"profile"`
			}
			if json.Unmarshal(body, &parsed) == nil && (parsed.Profile == "balanced" || parsed.Profile == "fiber4k") {
				pcWifiProfile = parsed.Profile
			}
		}
		writeJSON(w, 200, map[string]string{"status": "saved"})
		return
	}
	data, err := os.ReadFile(pcStateFile)
	if err != nil {
		writeJSON(w, 200, map[string]interface{}{
			"profile":          pcWifiProfile,
			"continueWatching": []interface{}{},
			"myList":           []interface{}{},
		})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

func (e *Engine) handlePcBootstrap(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"token":         pcActiveToken,
		"port":          pcListenPort,
		"ffmpegReady":   detectFfmpegBinary() != "",
		"wifiProfile":   pcWifiProfile,
		"engineVersion": "3.2-PC-NOCTURNE",
	})
}

func killAllFfmpegProcs() {
	pcFfmpegProcsMu.Lock()
	defer pcFfmpegProcsMu.Unlock()
	for slot, cmd := range pcFfmpegProcs {
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		delete(pcFfmpegProcs, slot)
	}
}
