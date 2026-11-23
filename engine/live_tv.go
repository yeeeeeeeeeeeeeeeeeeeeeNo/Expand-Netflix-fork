package main

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

type LiveBackupFeed struct {
	URL      string `json:"url"`
	Source   string `json:"source"`
	Quality  string `json:"quality"`
	Protocol string `json:"protocol"`
}

type LiveChannel struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	CleanKey string           `json:"clean_key"`
	Logo     string           `json:"logo"`
	Group    string           `json:"group"`
	Quality  string           `json:"quality"`
	Source   string           `json:"source"`
	URL      string           `json:"url"`
	Protocol string           `json:"protocol"`
	Verified bool             `json:"verified"`
	Backups  []LiveBackupFeed `json:"backups"`
}

var (
	liveStoreMu     sync.RWMutex
	liveChannels    []LiveChannel
	liveInitialized bool
	globalLogoMap   map[string]string
	liveHttpClient  = &http.Client{
		Timeout: 14 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
)

var officialLogoRules = []struct {
	Keywords []string
	LogoURL  string
}{
	{[]string{"cartoon network", "cn arabia", "cn "}, "https://upload.wikimedia.org/wikipedia/commons/thumb/8/80/Cartoon_Network_2010_logo.svg/960px-Cartoon_Network_2010_logo.svg.png"},
	{[]string{"spacetoon", "سبيستون"}, "https://upload.wikimedia.org/wikipedia/en/2/2b/Spacetoon_logo.png"},
	{[]string{"mbc 3", "mbc3"}, "https://i.imgur.com/PVt8OPN.png"},
	{[]string{"mbc action"}, "https://i.imgur.com/OWZAghw.png"},
	{[]string{"mbc max"}, "https://i.imgur.com/A02CptP.png"},
	{[]string{"mbc 2", "mbc2"}, "https://i.imgur.com/n9mSHuP.png"},
	{[]string{"mbc 4", "mbc4"}, "https://i.imgur.com/7afBZh2.png"},
	{[]string{"mbc 1", "mbc1", "mbc ", "mbc drama", "mbc masr", "mbc 5"}, "https://i.imgur.com/CiA3plN.png"},
	{[]string{"nickelodeon", "nicktoons", "nick jr", "nick "}, "https://i.imgur.com/E84jnP8.png"},
	{[]string{"toyor", "طيور الجنة"}, "https://i.imgur.com/5j8z6cc.png"},
	{[]string{"majd", "مجد"}, "https://i.imgur.com/ZiI374j.png"},
	{[]string{"bein"}, "https://i.imgur.com/Vtk2cGI.png"},
	{[]string{"arryadia", "الرياضية"}, "https://i.imgur.com/XjzK3gZ.png"},
	{[]string{"2m", "دوزيم"}, "https://i.imgur.com/PJYTfHi.png"},
	{[]string{"al aoula", "alaoula", "الأولى"}, "https://i.imgur.com/nq53d2N.png"},
	{[]string{"nat geo", "national geographic", "ad nat geo"}, "https://upload.wikimedia.org/wikipedia/commons/thumb/f/fc/Natgeologo.svg/960px-Natgeologo.svg.png"},
	{[]string{"ad sport", "abu dhabi sport", "yas sport", "أبوظبي"}, "https://i.imgur.com/6BVWk8z.png"},
	{[]string{"real madrid"}, "https://i.imgur.com/5pMo7dL.png"},
	{[]string{"sky sport"}, "https://i.imgur.com/COAQP59.png"},
	{[]string{"espn"}, "https://upload.wikimedia.org/wikipedia/commons/thumb/2/2f/ESPN_wordmark.svg/960px-ESPN_wordmark.svg.png"},
	{[]string{"eurosport"}, "https://i.imgur.com/s4jDnJh.png"},
	{[]string{"dazn"}, "https://i.imgur.com/2Z2EmZF.png"},
	{[]string{"sport tv", "sporttv"}, "https://i.imgur.com/U8cHMOt.png"},
	{[]string{"tnt sport"}, "https://upload.wikimedia.org/wikipedia/commons/thumb/d/d3/TNT_Sports_2021_logo.svg/960px-TNT_Sports_2021_logo.svg.png"},
	{[]string{"al jazeera", "aljazeera", "الجزيرة"}, "https://upload.wikimedia.org/wikipedia/en/thumb/f/f2/Aljazeera_eng.svg/512px-Aljazeera_eng.svg.png"},
	{[]string{"al arabiya", "alarabiya", "العربية"}, "https://upload.wikimedia.org/wikipedia/commons/thumb/e/e0/Al-Arabiya_new_logo.svg/512px-Al-Arabiya_new_logo.svg.png"},
	{[]string{"rotana"}, "https://upload.wikimedia.org/wikipedia/commons/thumb/8/87/Rotana_logo.svg/512px-Rotana_logo.svg.png"},
	{[]string{"fox sport"}, "https://upload.wikimedia.org/wikipedia/commons/thumb/0/0c/FOX_Sports_logo.svg/512px-FOX_Sports_logo.svg.png"},
	{[]string{"canal+"}, "https://upload.wikimedia.org/wikipedia/commons/thumb/1/1a/Canal%2B.svg/512px-Canal%2B.svg.png"},
	{[]string{"disney"}, "https://upload.wikimedia.org/wikipedia/commons/thumb/d/d2/2019_Disney_Channel_logo.svg/512px-2019_Disney_Channel_logo.svg.png"},
	{[]string{"france 24", "france24"}, "https://upload.wikimedia.org/wikipedia/en/thumb/6/65/FRANCE_24_logo.svg/512px-FRANCE_24_logo.svg.png"},
	{[]string{"bbc"}, "https://upload.wikimedia.org/wikipedia/commons/thumb/4/41/BBC_Logo_2021.svg/512px-BBC_Logo_2021.svg.png"},
	{[]string{"cnn"}, "https://upload.wikimedia.org/wikipedia/commons/thumb/b/b1/CNN.svg/512px-CNN.svg.png"},
}

var (
	reTvgLogo     = regexp.MustCompile(`tvg-logo="([^"]*)"`)
	reGroupTitle  = regexp.MustCompile(`group-title="([^"]*)"`)
	reNonAlpha    = regexp.MustCompile(`[^a-z0-9]`)
	reNoiseTokens = regexp.MustCompile(`\b(hd|fhd|uhd|sd|4k|8k|oscam|tv|channel|live|ar|arabic|mena|usa|uk|fr|es|de|it|pt|tr|inter|plus|hevc|vip)\b`)
)

func isAndroidBitmapFriendly(u string) bool {
	if u == "" {
		return false
	}
	low := strings.ToLower(u)
	if !strings.HasPrefix(low, "http://") && !strings.HasPrefix(low, "https://") {
		return false
	}
	// Android BitmapFactory cannot decode raw .svg files (Wikimedia /thumb/.../960px-...svg.png IS a PNG!)
	if strings.HasSuffix(low, ".svg") || (strings.Contains(low, ".svg?") && !strings.Contains(low, ".png")) {
		return false
	}
	return true
}

func normalizeChannelForLogo(name string) string {
	low := strings.ToLower(name)
	low = strings.Split(strings.Split(low, "(")[0], "[")[0]
	low = reNoiseTokens.ReplaceAllString(low, " ")
	return reNonAlpha.ReplaceAllString(low, "")
}

func fetchGlobalLogoIndex() map[string]string {
	type logoEntry struct {
		Channel string `json:"channel"`
		URL     string `json:"url"`
	}
	type chanEntry struct {
		ID       string   `json:"id"`
		Name     string   `json:"name"`
		AltNames []string `json:"alt_names"`
	}

	var logos []logoEntry
	var chans []chanEntry
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		req, err := http.NewRequest("GET", "https://iptv-org.github.io/api/logos.json", nil)
		if err != nil {
			return
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")
		resp, err := liveHttpClient.Do(req)
		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				resp.Body.Close()
			}
			return
		}
		defer resp.Body.Close()
		_ = json.NewDecoder(io.LimitReader(resp.Body, 12<<20)).Decode(&logos)
	}()

	go func() {
		defer wg.Done()
		req, err := http.NewRequest("GET", "https://iptv-org.github.io/api/channels.json", nil)
		if err != nil {
			return
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")
		resp, err := liveHttpClient.Do(req)
		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				resp.Body.Close()
			}
			return
		}
		defer resp.Body.Close()
		_ = json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&chans)
	}()

	wg.Wait()

	idToLogo := make(map[string]string, len(logos))
	for _, l := range logos {
		if l.Channel == "" || !isAndroidBitmapFriendly(l.URL) {
			continue
		}
		if _, exists := idToLogo[l.Channel]; !exists {
			idToLogo[l.Channel] = l.URL
		}
	}

	nameMap := make(map[string]string, len(chans)*2)
	for _, c := range chans {
		logoURL, ok := idToLogo[c.ID]
		if !ok {
			continue
		}
		baseID := normalizeChannelForLogo(strings.Split(c.ID, ".")[0])
		if len(baseID) >= 2 {
			if _, exists := nameMap[baseID]; !exists {
				nameMap[baseID] = logoURL
			}
		}
		kn := normalizeChannelForLogo(c.Name)
		if len(kn) >= 2 {
			if _, exists := nameMap[kn]; !exists {
				nameMap[kn] = logoURL
			}
		}
		for _, alt := range c.AltNames {
			ka := normalizeChannelForLogo(alt)
			if len(ka) >= 2 {
				if _, exists := nameMap[ka]; !exists {
					nameMap[ka] = logoURL
				}
			}
		}
	}

	// Also map channel IDs directly from logos.json even if missing from channels.json
	for cid, logoURL := range idToLogo {
		baseID := normalizeChannelForLogo(strings.Split(cid, ".")[0])
		if len(baseID) >= 2 {
			if _, exists := nameMap[baseID]; !exists {
				nameMap[baseID] = logoURL
			}
		}
	}
	return nameMap
}

func resolveOfficialLogo(name string, existing string, dir map[string]string) string {
	low := strings.ToLower(name)
	for _, rule := range officialLogoRules {
		for _, kw := range rule.Keywords {
			if strings.Contains(low, kw) {
				return rule.LogoURL
			}
		}
	}
	if isAndroidBitmapFriendly(existing) {
		return existing
	}
	if dir != nil {
		k := normalizeChannelForLogo(name)
		if len(k) >= 2 {
			if hit, ok := dir[k]; ok {
				return hit
			}
		}
		rawKey := reNonAlpha.ReplaceAllString(strings.Split(strings.Split(low, "(")[0], "[")[0], "")
		if len(rawKey) >= 2 {
			if hit, ok := dir[rawKey]; ok {
				return hit
			}
		}
	}
	return ""
}

func getVipSeedChannels() []LiveChannel {
	return []LiveChannel{
		{
			ID:       "vip_cn_arabia",
			Name:     "Cartoon Network Arabic (CN Arabia)",
			CleanKey: "cartoonnetworkarabic",
			Logo:     "https://upload.wikimedia.org/wikipedia/commons/thumb/8/80/Cartoon_Network_2010_logo.svg/960px-Cartoon_Network_2010_logo.svg.png",
			Group:    "Cartoon & Kids",
			Quality:  "1080p FHD",
			Source:   "Nilesat Astra Headend",
			URL:      "http://149.100.11.252:8000/play/a056/index.m3u8",
			Protocol: "FFMPEG",
			Verified: true,
			Backups: []LiveBackupFeed{
				{URL: "http://149.100.11.252:8000/play/a056/index.m3u8", Source: "Nilesat Direct Satellite", Quality: "1080p FHD", Protocol: "FFMPEG"},
				{URL: "http://167.249.23.102:8000/play/CartoonNetwork/index.m3u8", Source: "Astra CN Global Headend", Quality: "1080p FHD", Protocol: "FFMPEG"},
				{URL: "http://176.65.153.40:6002/play/a013/index.m3u8", Source: "Astra CN Turkiye Headend", Quality: "1080p FHD", Protocol: "FFMPEG"},
			},
		},
		{
			ID:       "vip_bein_xtra_1",
			Name:     "beIN SPORTS XTRA 1 (1080p 60fps)",
			CleanKey: "beinsportsxtra1",
			Logo:     "https://i.imgur.com/Vtk2cGI.png",
			Group:    "beIN & Sports VIP",
			Quality:  "1080p FHD",
			Source:   "Amagi Direct 1080p60 CDN",
			URL:      "https://bein-xtra-bein.amagi.tv/playlist.m3u8",
			Protocol: "HLS",
			Verified: true,
			Backups: []LiveBackupFeed{
				{URL: "https://bein-xtra-bein.amagi.tv/playlist.m3u8", Source: "Amagi Direct 1080p60", Quality: "1080p FHD", Protocol: "HLS"},
				{URL: "https://jmp2.uk/stvp-CABA8000043F", Source: "Samsung TV+ 1080p", Quality: "1080p FHD", Protocol: "HLS"},
			},
		},
		{
			ID:       "vip_bein_xtra_2",
			Name:     "beIN SPORTS XTRA 2 / LaLiga & Football",
			CleanKey: "beinsportsxtra2",
			Logo:     "https://i.imgur.com/Vtk2cGI.png",
			Group:    "beIN & Sports VIP",
			Quality:  "1080p FHD",
			Source:   "AWS CloudFront 1080p60",
			URL:      "https://dc1644a9jazgj.cloudfront.net/beIN_Sports_Xtra_Espanol.m3u8",
			Protocol: "HLS",
			Verified: true,
			Backups: []LiveBackupFeed{
				{URL: "https://dc1644a9jazgj.cloudfront.net/beIN_Sports_Xtra_Espanol.m3u8", Source: "AWS CloudFront 1080p60", Quality: "1080p FHD", Protocol: "HLS"},
			},
		},
		{
			ID:       "vip_bein_1_live",
			Name:     "beIN SPORTS 1 FHD Live",
			CleanKey: "beinsports1live",
			Logo:     "https://i.imgur.com/Vtk2cGI.png",
			Group:    "beIN & Sports VIP",
			Quality:  "1080p FHD",
			Source:   "AynaOTT Direct + MegaTV FHD",
			URL:      "https://tvsen6.aynaott.com/beinsports/index.m3u8",
			Protocol: "HLS",
			Verified: true,
			Backups: []LiveBackupFeed{
				{URL: "https://tvsen6.aynaott.com/beinsports/index.m3u8", Source: "AynaOTT Direct HD", Quality: "1080p FHD", Protocol: "HLS"},
				{URL: "http://megatv5074.com/live/81NQAFA7Q5S4188/JWyIZeiu08/348325.ts", Source: "MegaTV beIN Sports 1 English FHD", Quality: "1080p FHD", Protocol: "FFMPEG"},
				{URL: "http://megatv5074.com/live/81NQAFA7Q5S4188/JWyIZeiu08/56655.ts", Source: "MegaTV beIN Sports 1 Arabic FHD", Quality: "1080p FHD", Protocol: "FFMPEG"},
			},
		},
		{
			ID:       "vip_spacetoon_ar",
			Name:     "Spacetoon Arabic (سبيستون)",
			CleanKey: "spacetoonarabic",
			Logo:     "https://upload.wikimedia.org/wikipedia/en/2/2b/Spacetoon_logo.png",
			Group:    "Cartoon & Kids",
			Quality:  "1080p FHD",
			Source:   "SpacetoonGo MENA Official",
			URL:      "https://live-uae-next.spacetoongo.com/ST_MENA_NEXT/hls/r9p2hjipmw2kl.m3u8",
			Protocol: "HLS",
			Verified: true,
			Backups: []LiveBackupFeed{
				{URL: "https://live-uae-next.spacetoongo.com/ST_MENA_NEXT/hls/r9p2hjipmw2kl.m3u8", Source: "SpacetoonGo MENA Official", Quality: "1080p FHD", Protocol: "HLS"},
				{URL: "https://shd-gcp-live.edgenextcdn.net/live/bitmovin-spacetoon/d8382fb9ab4b2307058f12c7ea90db54/index.m3u8", Source: "EdgeNext 1080p CDN", Quality: "1080p FHD", Protocol: "HLS"},
			},
		},
		{
			ID:       "vip_mbc3_ar",
			Name:     "MBC 3 Arabic Kids (إم بي سي 3)",
			CleanKey: "mbc3arabic",
			Logo:     "https://i.imgur.com/PVt8OPN.png",
			Group:    "Cartoon & Kids",
			Quality:  "1080p FHD",
			Source:   "Nilesat Astra Headend",
			URL:      "http://149.100.11.252:8000/play/a028/index.m3u8",
			Protocol: "FFMPEG",
			Verified: true,
			Backups: []LiveBackupFeed{
				{URL: "http://149.100.11.252:8000/play/a028/index.m3u8", Source: "Nilesat Direct Satellite", Quality: "1080p FHD", Protocol: "FFMPEG"},
				{URL: "https://shd-gcp-live.edgenextcdn.net/live/bitmovin-mbc-3-usa/5d58265a862a476dc7f97694addb5ded/index.m3u8", Source: "EdgeNext 1080p CDN", Quality: "1080p FHD", Protocol: "HLS"},
			},
		},
		{
			ID:       "vip_nick_ar",
			Name:     "Nickelodeon Arabia (نيكلوديون بالعربية)",
			CleanKey: "nickelodeonarabia",
			Logo:     "https://i.imgur.com/E84jnP8.png",
			Group:    "Cartoon & Kids",
			Quality:  "1080p FHD",
			Source:   "MENA Satellite Headend",
			URL:      "http://85.237.89.159:3342/ar-s/Nickelodeon/index.m3u8",
			Protocol: "FFMPEG",
			Verified: true,
			Backups: []LiveBackupFeed{
				{URL: "http://85.237.89.159:3342/ar-s/Nickelodeon/index.m3u8", Source: "MENA Satellite Headend", Quality: "1080p FHD", Protocol: "FFMPEG"},
				{URL: "https://jmp2.uk/plu-5ca673e0d0bd6c2689c94ce3.m3u8", Source: "Pluto TV Nickelodeon", Quality: "1080p FHD", Protocol: "HLS"},
			},
		},
		{
			ID:       "vip_arryadia_ma",
			Name:     "Arryadia Morocco Sports HD (الرياضية المغربية)",
			CleanKey: "arryadiamoroccosports",
			Logo:     "https://i.imgur.com/XjzK3gZ.png",
			Group:    "beIN & Sports VIP",
			Quality:  "1080p FHD",
			Source:   "Nilesat Astra Headend",
			URL:      "http://149.100.11.252:8000/play/a065/index.m3u8",
			Protocol: "FFMPEG",
			Verified: true,
			Backups: []LiveBackupFeed{
				{URL: "http://149.100.11.252:8000/play/a065/index.m3u8", Source: "Arryadia HD (Nilesat)", Quality: "1080p HD", Protocol: "FFMPEG"},
				{URL: "http://149.100.11.252:8000/play/a04o/index.m3u8", Source: "El Heddaf Sports TV", Quality: "1080p HD", Protocol: "FFMPEG"},
			},
		},
		{
			ID:       "vip_adsports_hd",
			Name:     "Abu Dhabi Sports 1/2 & YAS Sports HD",
			CleanKey: "abudhabisports12hd",
			Logo:     "https://i.imgur.com/6BVWk8z.png",
			Group:    "beIN & Sports VIP",
			Quality:  "1080p FHD",
			Source:   "Nilesat Astra Headend",
			URL:      "http://149.100.11.252:8000/play/a00p/index.m3u8",
			Protocol: "FFMPEG",
			Verified: true,
			Backups: []LiveBackupFeed{
				{URL: "http://149.100.11.252:8000/play/a00p/index.m3u8", Source: "AD Sport 1 HD (Nilesat)", Quality: "1080p FHD", Protocol: "FFMPEG"},
				{URL: "http://149.100.11.252:8000/play/a00q/index.m3u8", Source: "AD Sport 2 HD (Nilesat)", Quality: "1080p FHD", Protocol: "FFMPEG"},
				{URL: "http://149.100.11.252:8000/play/a00r/index.m3u8", Source: "YAS Sports HD (Nilesat)", Quality: "1080p FHD", Protocol: "FFMPEG"},
			},
		},
		{
			ID:       "vip_realmadrid_tv",
			Name:     "Real Madrid TV (1080p FHD Live)",
			CleanKey: "realmadridtv",
			Logo:     "https://i.imgur.com/5pMo7dL.png",
			Group:    "beIN & Sports VIP",
			Quality:  "1080p FHD",
			Source:   "Spain 1080p Headend",
			URL:      "http://90.163.85.113:8093/stream/26/index.m3u8",
			Protocol: "FFMPEG",
			Verified: true,
			Backups: []LiveBackupFeed{
				{URL: "http://90.163.85.113:8093/stream/26/index.m3u8", Source: "Spain 1080p Headend", Quality: "1080p FHD", Protocol: "FFMPEG"},
				{URL: "http://88.212.15.50/live/real_madrid_tv/index.m3u8", Source: "Real Madrid TV English HD", Quality: "1080p FHD", Protocol: "HLS"},
			},
		},
		{
			ID:       "vip_skysports_espn",
			Name:     "Sky Sports & ESPN 1/2 HD",
			CleanKey: "skysportsespnhd",
			Logo:     "https://i.imgur.com/COAQP59.png",
			Group:    "beIN & Sports VIP",
			Quality:  "1080p FHD",
			Source:   "Astra Satellite Headend",
			URL:      "http://45.228.234.9:8000/play/a06b/index.m3u8",
			Protocol: "FFMPEG",
			Verified: true,
			Backups: []LiveBackupFeed{
				{URL: "http://45.228.234.9:8000/play/a06b/index.m3u8", Source: "Sky Sports 1 HD", Quality: "1080p FHD", Protocol: "FFMPEG"},
				{URL: "http://85.237.89.160:9590/usa-s/ESPN-HD/index.m3u8", Source: "ESPN 1 HD", Quality: "1080p HD", Protocol: "FFMPEG"},
				{URL: "http://85.237.89.160:9590/usa-s/ESPN-2-HD/index.m3u8", Source: "ESPN 2 HD", Quality: "1080p HD", Protocol: "FFMPEG"},
			},
		},
		{
			ID:       "vip_mbc_bouquet",
			Name:     "MBC 1 / MBC 2 / MBC Action / MBC Max",
			CleanKey: "mbcbouquet1080p",
			Logo:     "https://i.imgur.com/CiA3plN.png",
			Group:    "Arabic Satellite",
			Quality:  "1080p FHD",
			Source:   "Nilesat Astra Headend",
			URL:      "http://149.100.11.252:8000/play/a02a/index.m3u8",
			Protocol: "FFMPEG",
			Verified: true,
			Backups: []LiveBackupFeed{
				{URL: "http://149.100.11.252:8000/play/a02a/index.m3u8", Source: "MBC 2 Movies (Nilesat)", Quality: "1080p HD", Protocol: "FFMPEG"},
				{URL: "http://149.100.11.252:8000/play/a029/index.m3u8", Source: "MBC Action (Nilesat)", Quality: "1080p HD", Protocol: "FFMPEG"},
				{URL: "http://149.100.11.252:8000/play/a02b/index.m3u8", Source: "MBC Max (Nilesat)", Quality: "1080p HD", Protocol: "FFMPEG"},
			},
		},
		{
			ID:       "vip_morocco_bouquet",
			Name:     "2M Maroc & Al Aoula Inter HD (قنوات المغرب)",
			CleanKey: "moroccobouquet",
			Logo:     "https://i.imgur.com/PJYTfHi.png",
			Group:    "Arabic Satellite",
			Quality:  "1080p FHD",
			Source:   "Nilesat Astra Headend",
			URL:      "http://149.100.11.252:8000/play/a064/index.m3u8",
			Protocol: "FFMPEG",
			Verified: true,
			Backups: []LiveBackupFeed{
				{URL: "http://149.100.11.252:8000/play/a064/index.m3u8", Source: "2M TV Maroc (Nilesat)", Quality: "1080p HD", Protocol: "FFMPEG"},
				{URL: "http://149.100.11.252:8000/play/a063/index.m3u8", Source: "Al Aoula Inter HD", Quality: "1080p FHD", Protocol: "FFMPEG"},
			},
		},
		{
			ID:       "vip_natgeo_ad",
			Name:     "National Geographic Abu Dhabi HD",
			CleanKey: "natgeoabudhabi",
			Logo:     "https://upload.wikimedia.org/wikipedia/commons/thumb/f/fc/Natgeologo.svg/960px-Natgeologo.svg.png",
			Group:    "Arabic Satellite",
			Quality:  "1080p FHD",
			Source:   "Nilesat Astra Headend",
			URL:      "http://149.100.11.252:8000/play/a020/index.m3u8",
			Protocol: "FFMPEG",
			Verified: true,
			Backups: []LiveBackupFeed{
				{URL: "http://149.100.11.252:8000/play/a020/index.m3u8", Source: "AD Nat Geo HD (Nilesat)", Quality: "1080p FHD", Protocol: "FFMPEG"},
			},
		},
	}
}

func parseM3U(rawText string, sourceTag string, categoryOverride string, logoDir map[string]string) []LiveChannel {
	var out []LiveChannel
	lines := strings.Split(rawText, "\n")
	var cur *LiveChannel

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#EXTINF:") {
			logo := ""
			if m := reTvgLogo.FindStringSubmatch(line); len(m) > 1 {
				logo = m[1]
			}
			grp := categoryOverride
			if grp == "" {
				if m := reGroupTitle.FindStringSubmatch(line); len(m) > 1 {
					grp = strings.TrimSpace(m[1])
				}
			}
			if grp == "" || grp == "-" {
				grp = "General"
			}
			parts := strings.Split(line, ",")
			name := strings.TrimSpace(parts[len(parts)-1])
			if name == "" {
				name = "Channel"
			}
			upper := strings.ToUpper(name)
			qual := "HD"
			if strings.Contains(upper, "4K") || strings.Contains(upper, "UHD") || strings.Contains(upper, "2160P") {
				qual = "4K UHD"
			} else if strings.Contains(upper, "1080") || strings.Contains(upper, "FHD") || strings.Contains(upper, "HD") {
				qual = "1080p FHD"
			}
			low := strings.ToLower(name)
			if strings.Contains(low, "bein") || strings.Contains(low, "sport") || strings.Contains(low, "arryadia") || strings.Contains(low, "espn") || strings.Contains(low, "dazn") || strings.Contains(low, "eurosport") {
				grp = "beIN & Sports VIP"
			} else if strings.Contains(low, "cartoon") || strings.Contains(low, "cn ") || strings.Contains(low, "spacetoon") || strings.Contains(low, "mbc 3") || strings.Contains(low, "nick") || strings.Contains(low, "kids") || strings.Contains(low, "toyor") || strings.Contains(low, "majd") {
				grp = "Cartoon & Kids"
			}
			cleanBase := strings.Split(strings.Split(low, "(")[0], "[")[0]
			cleanKey := reNonAlpha.ReplaceAllString(cleanBase, "")
			logo = resolveOfficialLogo(name, logo, logoDir)

			cur = &LiveChannel{
				Name:     name,
				CleanKey: cleanKey,
				Logo:     logo,
				Group:    grp,
				Quality:  qual,
				Source:   sourceTag,
				Protocol: "HLS",
			}
		} else if !strings.HasPrefix(line, "#") && cur != nil {
			if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
				if strings.Contains(line, "jmp2.uk/stvp-US") {
					cur = nil
					continue
				}
				cur.URL = line
				lowU := strings.ToLower(line)
				if strings.Contains(lowU, "/play/") || strings.Contains(lowU, ":8000") || strings.Contains(lowU, ":8201") || strings.HasSuffix(lowU, ".ts") {
					cur.Protocol = "FFMPEG"
				}
				cur.Backups = []LiveBackupFeed{{
					URL:      cur.URL,
					Source:   cur.Source,
					Quality:  cur.Quality,
					Protocol: cur.Protocol,
				}}
				out = append(out, *cur)
				cur = nil
			}
		}
	}
	return out
}

func ensureLiveHarvested() {
	liveStoreMu.RLock()
	if liveInitialized {
		liveStoreMu.RUnlock()
		return
	}
	liveStoreMu.RUnlock()

	sources := []struct {
		Name string
		URL  string
		Cat  string
	}{
		{"Nilesat Astra Headend", "http://149.100.11.252:8000/playlist.m3u8", "Arabic Satellite"},
		{"OSCam Sports Headend", "http://176.65.153.78:8201/playlist.m3u8", "beIN & Sports VIP"},
		{"BuddyChewChew Samsung TV+", "https://raw.githubusercontent.com/BuddyChewChew/app-m3u-generator/main/playlists/samsungtvplus_all.m3u", "Samsung TV+"},
		{"Global Sports Live", "https://iptv-org.github.io/iptv/categories/sports.m3u", "beIN & Sports VIP"},
		{"Global Kids & Cartoon", "https://iptv-org.github.io/iptv/categories/kids.m3u", "Cartoon & Kids"},
	}

	// Fetch 35,676 global logos in parallel with the M3U playlists!
	logoDirCh := make(chan map[string]string, 1)
	go func() {
		logoDirCh <- fetchGlobalLogoIndex()
	}()

	type rawM3URes struct {
		Name string
		Cat  string
		Text string
	}
	rawCh := make(chan rawM3URes, len(sources))
	for _, s := range sources {
		go func(name, u, cat string) {
			req, err := http.NewRequest("GET", u, nil)
			if err != nil {
				rawCh <- rawM3URes{}
				return
			}
			req.Header.Set("User-Agent", "TiviMate/4.7.0 (Linux; Android 11; Shield Android TV)")
			resp, err := liveHttpClient.Do(req)
			if err != nil || resp.StatusCode != 200 {
				if resp != nil {
					resp.Body.Close()
				}
				rawCh <- rawM3URes{}
				return
			}
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 6<<20))
			resp.Body.Close()
			rawCh <- rawM3URes{Name: name, Cat: cat, Text: string(body)}
		}(s.Name, s.URL, s.Cat)
	}

	logoDir := <-logoDirCh
	var rawResults []rawM3URes
	for i := 0; i < len(sources); i++ {
		r := <-rawCh
		if r.Text != "" {
			rawResults = append(rawResults, r)
		}
	}

	bonded := make(map[string]*LiveChannel)
	var order []string
	for _, vip := range getVipSeedChannels() {
		v := vip
		bonded[v.CleanKey] = &v
		order = append(order, v.CleanKey)
	}

	for _, r := range rawResults {
		items := parseM3U(r.Text, r.Name, r.Cat, logoDir)
		for _, item := range items {
			key := item.CleanKey
			if len(key) < 3 {
				key = strings.ToLower(item.Name)
			}
			if existing, ok := bonded[key]; ok {
				if existing.Logo == "" && item.Logo != "" {
					existing.Logo = item.Logo
				}
				dup := false
				for _, b := range existing.Backups {
					if b.URL == item.URL {
						dup = true
						break
					}
				}
				if !dup {
					existing.Backups = append(existing.Backups, LiveBackupFeed{
						URL:      item.URL,
						Source:   item.Source,
						Quality:  item.Quality,
						Protocol: item.Protocol,
					})
				}
			} else {
				cp := item
				cp.ID = "ch_" + key
				bonded[key] = &cp
				order = append(order, key)
			}
		}
	}

	liveStoreMu.Lock()
	globalLogoMap = logoDir
	liveChannels = make([]LiveChannel, 0, len(order))
	// Sort so channels WITH verified real logos surface cleanly first inside each category!
	var withLogo []LiveChannel
	var noLogo []LiveChannel
	for _, k := range order {
		if c, ok := bonded[k]; ok {
			if strings.HasPrefix(c.ID, "vip_") || c.Logo != "" {
				withLogo = append(withLogo, *c)
			} else {
				noLogo = append(noLogo, *c)
			}
		}
	}
	liveChannels = append(liveChannels, withLogo...)
	liveChannels = append(liveChannels, noLogo...)
	liveInitialized = true
	liveStoreMu.Unlock()
}

func (e *Engine) handleLiveChannels(w http.ResponseWriter, r *http.Request) {
	ensureLiveHarvested()
	groupFilter := strings.TrimSpace(r.URL.Query().Get("group"))
	if groupFilter == "" {
		groupFilter = "VIP"
	}
	searchFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))

	liveStoreMu.RLock()
	defer liveStoreMu.RUnlock()

	var results []LiveChannel
	for _, ch := range liveChannels {
		if groupFilter == "VIP" {
			if !strings.HasPrefix(ch.ID, "vip_") && ch.Group != "beIN & Sports VIP" && ch.Group != "Cartoon & Kids" {
				continue
			}
		} else if groupFilter != "ALL" && !strings.EqualFold(ch.Group, groupFilter) {
			continue
		}
		if searchFilter != "" && !strings.Contains(strings.ToLower(ch.Name), searchFilter) && !strings.Contains(strings.ToLower(ch.Group), searchFilter) {
			continue
		}
		results = append(results, ch)
		if len(results) >= 400 {
			break
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total":    len(liveChannels),
		"returned": len(results),
		"channels": results,
	})
}

func (e *Engine) handleLiveProxy(w http.ResponseWriter, r *http.Request) {
	rawURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if rawURL == "" {
		http.Error(w, "Missing url", http.StatusBadRequest)
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), "GET", rawURL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.Header.Set("User-Agent", "TiviMate/4.7.0 (Linux; Android 11; Shield Android TV)")
	req.Header.Set("Accept", "*/*")

	resp, err := liveHttpClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL.String()
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(strings.ToLower(finalURL), ".m3u8") || strings.Contains(ct, "mpegurl") || strings.Contains(rawURL, "jmp2.uk") {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		trimmed := strings.TrimSpace(string(body))
		if strings.HasPrefix(trimmed, "#EXTM3U") {
			baseU, _ := url.Parse(finalURL)
			var outLines []string
			for _, line := range strings.Split(trimmed, "\n") {
				sline := strings.TrimSpace(line)
				if sline == "" {
					continue
				}
				if strings.HasPrefix(sline, "#") {
					outLines = append(outLines, sline)
				} else {
					segURL := sline
					if !strings.HasPrefix(segURL, "http://") && !strings.HasPrefix(segURL, "https://") && baseU != nil {
						if ref, err := url.Parse(segURL); err == nil {
							segURL = baseU.ResolveReference(ref).String()
						}
					}
					outLines = append(outLines, "/live/proxy?url="+url.QueryEscape(segURL))
				}
			}
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(strings.Join(outLines, "\n")))
			return
		}
		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(body)
		return
	}

	if ctype := resp.Header.Get("Content-Type"); ctype != "" {
		w.Header().Set("Content-Type", ctype)
	} else {
		w.Header().Set("Content-Type", "video/mp2t")
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
