package com.d4cx.streamflix

import org.json.JSONArray
import org.json.JSONObject
import java.io.BufferedReader
import java.io.InputStreamReader
import java.net.HttpURLConnection
import java.net.URL
import java.net.URLEncoder
import java.util.Locale

object LiveTvRepository {

    private val officialLogos = listOf(
        listOf("cartoon network", "cn arabia", "cn ") to "https://upload.wikimedia.org/wikipedia/commons/thumb/8/80/Cartoon_Network_2010_logo.svg/960px-Cartoon_Network_2010_logo.svg.png",
        listOf("spacetoon", "سبيستون") to "https://upload.wikimedia.org/wikipedia/en/2/2b/Spacetoon_logo.png",
        listOf("mbc 3", "mbc3") to "https://i.imgur.com/PVt8OPN.png",
        listOf("mbc action") to "https://i.imgur.com/OWZAghw.png",
        listOf("mbc max") to "https://i.imgur.com/A02CptP.png",
        listOf("mbc 2", "mbc2") to "https://i.imgur.com/n9mSHuP.png",
        listOf("mbc 1", "mbc1", "mbc ") to "https://i.imgur.com/CiA3plN.png",
        listOf("nickelodeon", "nicktoons", "nick ") to "https://i.imgur.com/E84jnP8.png",
        listOf("toyor", "طيور الجنة") to "https://i.imgur.com/5j8z6cc.png",
        listOf("majd", "مجد") to "https://i.imgur.com/ZiI374j.png",
        listOf("bein") to "https://i.imgur.com/Vtk2cGI.png",
        listOf("arryadia", "الرياضية") to "https://i.imgur.com/XjzK3gZ.png",
        listOf("2m", "دوزيم") to "https://i.imgur.com/PJYTfHi.png",
        listOf("al aoula", "alaoula", "الأولى") to "https://i.imgur.com/nq53d2N.png",
        listOf("ad sport", "abu dhabi", "yas sport", "أبوظبي") to "https://i.imgur.com/6BVWk8z.png",
        listOf("real madrid") to "https://i.imgur.com/5pMo7dL.png",
        listOf("sky sport") to "https://i.imgur.com/COAQP59.png",
        listOf("espn") to "https://upload.wikimedia.org/wikipedia/commons/thumb/2/2f/ESPN_wordmark.svg/960px-ESPN_wordmark.svg.png",
        listOf("eurosport") to "https://i.imgur.com/s4jDnJh.png",
        listOf("dazn") to "https://i.imgur.com/2Z2EmZF.png",
        listOf("sport tv") to "https://i.imgur.com/U8cHMOt.png",
        listOf("tnt sport") to "https://upload.wikimedia.org/wikipedia/commons/thumb/d/d3/TNT_Sports_2021_logo.svg/960px-TNT_Sports_2021_logo.svg.png"
    )

    fun resolveLogo(name: String, existing: String = ""): String {
        val low = name.lowercase(Locale.US)
        for ((keys, logoUrl) in officialLogos) {
            if (keys.any { low.contains(it) }) return logoUrl
        }
        return existing
    }

    fun getBuiltInVipChannels(): List<LiveChannelItem> = listOf(
        LiveChannelItem(
            id = "vip_cn_arabia",
            name = "Cartoon Network Arabic (CN Arabia)",
            logo = "https://upload.wikimedia.org/wikipedia/commons/thumb/8/80/Cartoon_Network_2010_logo.svg/960px-Cartoon_Network_2010_logo.svg.png",
            group = "Cartoon & Kids",
            quality = "1080p FHD",
            source = "Nilesat Astra Headend",
            url = "http://149.100.11.252:8000/play/a056/index.m3u8",
            verified = true,
            backups = listOf(
                LiveFeedItem("http://149.100.11.252:8000/play/a056/index.m3u8", "Nilesat Direct Satellite", "1080p FHD"),
                LiveFeedItem("http://167.249.23.102:8000/play/CartoonNetwork/index.m3u8", "Astra CN Global Headend", "1080p FHD"),
                LiveFeedItem("http://176.65.153.40:6002/play/a013/index.m3u8", "Astra CN Turkiye Headend", "1080p FHD")
            )
        ),
        LiveChannelItem(
            id = "vip_bein_xtra_1",
            name = "beIN SPORTS XTRA 1 (1080p 60fps)",
            logo = "https://i.imgur.com/Vtk2cGI.png",
            group = "beIN & Sports VIP",
            quality = "1080p FHD",
            source = "Amagi Direct 1080p60 CDN",
            url = "https://bein-xtra-bein.amagi.tv/playlist.m3u8",
            verified = true,
            backups = listOf(
                LiveFeedItem("https://bein-xtra-bein.amagi.tv/playlist.m3u8", "Amagi Direct 1080p60", "1080p FHD"),
                LiveFeedItem("https://jmp2.uk/stvp-CABA8000043F", "Samsung TV+ 1080p", "1080p FHD")
            )
        ),
        LiveChannelItem(
            id = "vip_bein_xtra_2",
            name = "beIN SPORTS XTRA 2 / LaLiga & Football",
            logo = "https://i.imgur.com/Vtk2cGI.png",
            group = "beIN & Sports VIP",
            quality = "1080p FHD",
            source = "AWS CloudFront 1080p60",
            url = "https://dc1644a9jazgj.cloudfront.net/beIN_Sports_Xtra_Espanol.m3u8",
            verified = true,
            backups = listOf(
                LiveFeedItem("https://dc1644a9jazgj.cloudfront.net/beIN_Sports_Xtra_Espanol.m3u8", "AWS CloudFront 1080p60", "1080p FHD")
            )
        ),
        LiveChannelItem(
            id = "vip_bein_1_live",
            name = "beIN SPORTS 1 FHD Live",
            logo = "https://i.imgur.com/Vtk2cGI.png",
            group = "beIN & Sports VIP",
            quality = "1080p FHD",
            source = "AynaOTT Direct + MegaTV FHD",
            url = "https://tvsen6.aynaott.com/beinsports/index.m3u8",
            verified = true,
            backups = listOf(
                LiveFeedItem("https://tvsen6.aynaott.com/beinsports/index.m3u8", "AynaOTT Direct HD", "1080p FHD"),
                LiveFeedItem("http://megatv5074.com/live/81NQAFA7Q5S4188/JWyIZeiu08/348325.ts", "MegaTV beIN Sports 1 English FHD", "1080p FHD"),
                LiveFeedItem("http://megatv5074.com/live/81NQAFA7Q5S4188/JWyIZeiu08/56655.ts", "MegaTV beIN Sports 1 Arabic FHD", "1080p FHD")
            )
        ),
        LiveChannelItem(
            id = "vip_spacetoon_ar",
            name = "Spacetoon Arabic (سبيستون)",
            logo = "https://upload.wikimedia.org/wikipedia/en/2/2b/Spacetoon_logo.png",
            group = "Cartoon & Kids",
            quality = "1080p FHD",
            source = "SpacetoonGo MENA Official",
            url = "https://live-uae-next.spacetoongo.com/ST_MENA_NEXT/hls/r9p2hjipmw2kl.m3u8",
            verified = true,
            backups = listOf(
                LiveFeedItem("https://live-uae-next.spacetoongo.com/ST_MENA_NEXT/hls/r9p2hjipmw2kl.m3u8", "SpacetoonGo MENA Official", "1080p FHD"),
                LiveFeedItem("https://shd-gcp-live.edgenextcdn.net/live/bitmovin-spacetoon/d8382fb9ab4b2307058f12c7ea90db54/index.m3u8", "EdgeNext 1080p CDN", "1080p FHD")
            )
        ),
        LiveChannelItem(
            id = "vip_mbc3_ar",
            name = "MBC 3 Arabic Kids (إم بي سي 3)",
            logo = "https://i.imgur.com/PVt8OPN.png",
            group = "Cartoon & Kids",
            quality = "1080p FHD",
            source = "Nilesat Astra Headend",
            url = "http://149.100.11.252:8000/play/a028/index.m3u8",
            verified = true,
            backups = listOf(
                LiveFeedItem("http://149.100.11.252:8000/play/a028/index.m3u8", "Nilesat Direct Satellite", "1080p FHD"),
                LiveFeedItem("https://shd-gcp-live.edgenextcdn.net/live/bitmovin-mbc-3-usa/5d58265a862a476dc7f97694addb5ded/index.m3u8", "EdgeNext 1080p CDN", "1080p FHD")
            )
        ),
        LiveChannelItem(
            id = "vip_nick_ar",
            name = "Nickelodeon Arabia (نيكلوديون بالعربية)",
            logo = "https://i.imgur.com/E84jnP8.png",
            group = "Cartoon & Kids",
            quality = "1080p FHD",
            source = "MENA Satellite Headend",
            url = "http://85.237.89.159:3342/ar-s/Nickelodeon/index.m3u8",
            verified = true,
            backups = listOf(
                LiveFeedItem("http://85.237.89.159:3342/ar-s/Nickelodeon/index.m3u8", "MENA Satellite Headend", "1080p FHD"),
                LiveFeedItem("https://jmp2.uk/plu-5ca673e0d0bd6c2689c94ce3.m3u8", "Pluto TV Nickelodeon", "1080p FHD")
            )
        ),
        LiveChannelItem(
            id = "vip_arryadia_ma",
            name = "Arryadia Morocco Sports HD (الرياضية المغربية)",
            logo = "https://i.imgur.com/XjzK3gZ.png",
            group = "beIN & Sports VIP",
            quality = "1080p FHD",
            source = "Nilesat Astra Headend",
            url = "http://149.100.11.252:8000/play/a065/index.m3u8",
            verified = true,
            backups = listOf(
                LiveFeedItem("http://149.100.11.252:8000/play/a065/index.m3u8", "Arryadia HD (Nilesat)", "1080p FHD"),
                LiveFeedItem("http://149.100.11.252:8000/play/a04o/index.m3u8", "El Heddaf Sports TV", "1080p HD")
            )
        ),
        LiveChannelItem(
            id = "vip_adsports_hd",
            name = "Abu Dhabi Sports 1/2 & YAS Sports HD",
            logo = "https://i.imgur.com/6BVWk8z.png",
            group = "beIN & Sports VIP",
            quality = "1080p FHD",
            source = "Nilesat Astra Headend",
            url = "http://149.100.11.252:8000/play/a00p/index.m3u8",
            verified = true,
            backups = listOf(
                LiveFeedItem("http://149.100.11.252:8000/play/a00p/index.m3u8", "AD Sport 1 HD", "1080p FHD"),
                LiveFeedItem("http://149.100.11.252:8000/play/a00q/index.m3u8", "AD Sport 2 HD", "1080p FHD"),
                LiveFeedItem("http://149.100.11.252:8000/play/a00r/index.m3u8", "YAS Sports HD", "1080p FHD")
            )
        ),
        LiveChannelItem(
            id = "vip_realmadrid_tv",
            name = "Real Madrid TV (1080p FHD Live)",
            logo = "https://i.imgur.com/5pMo7dL.png",
            group = "beIN & Sports VIP",
            quality = "1080p FHD",
            source = "Spain 1080p Headend",
            url = "http://90.163.85.113:8093/stream/26/index.m3u8",
            verified = true,
            backups = listOf(
                LiveFeedItem("http://90.163.85.113:8093/stream/26/index.m3u8", "Spain 1080p Headend", "1080p FHD"),
                LiveFeedItem("http://88.212.15.50/live/real_madrid_tv/index.m3u8", "Real Madrid TV English HD", "1080p FHD")
            )
        ),
        LiveChannelItem(
            id = "vip_skysports_espn",
            name = "Sky Sports & ESPN 1/2 HD",
            logo = "https://i.imgur.com/COAQP59.png",
            group = "beIN & Sports VIP",
            quality = "1080p FHD",
            source = "Astra Satellite Headend",
            url = "http://45.228.234.9:8000/play/a06b/index.m3u8",
            verified = true,
            backups = listOf(
                LiveFeedItem("http://45.228.234.9:8000/play/a06b/index.m3u8", "Sky Sports 1 HD", "1080p FHD"),
                LiveFeedItem("http://85.237.89.160:9590/usa-s/ESPN-HD/index.m3u8", "ESPN 1 HD", "1080p FHD"),
                LiveFeedItem("http://85.237.89.160:9590/usa-s/ESPN-2-HD/index.m3u8", "ESPN 2 HD", "1080p FHD")
            )
        ),
        LiveChannelItem(
            id = "vip_mbc_bouquet",
            name = "MBC 1 / MBC 2 / MBC Action / MBC Max",
            logo = "https://i.imgur.com/CiA3plN.png",
            group = "Arabic Satellite",
            quality = "1080p FHD",
            source = "Nilesat Astra Headend",
            url = "http://149.100.11.252:8000/play/a02a/index.m3u8",
            verified = true,
            backups = listOf(
                LiveFeedItem("http://149.100.11.252:8000/play/a02a/index.m3u8", "MBC 2 Movies (Nilesat)", "1080p FHD"),
                LiveFeedItem("http://149.100.11.252:8000/play/a029/index.m3u8", "MBC Action (Nilesat)", "1080p FHD"),
                LiveFeedItem("http://149.100.11.252:8000/play/a02b/index.m3u8", "MBC Max (Nilesat)", "1080p FHD")
            )
        ),
        LiveChannelItem(
            id = "vip_morocco_bouquet",
            name = "2M Maroc & Al Aoula Inter HD (قنوات المغرب)",
            logo = "https://i.imgur.com/PJYTfHi.png",
            group = "Arabic Satellite",
            quality = "1080p FHD",
            source = "Nilesat Astra Headend",
            url = "http://149.100.11.252:8000/play/a064/index.m3u8",
            verified = true,
            backups = listOf(
                LiveFeedItem("http://149.100.11.252:8000/play/a064/index.m3u8", "2M TV Maroc (Nilesat)", "1080p FHD"),
                LiveFeedItem("http://149.100.11.252:8000/play/a063/index.m3u8", "Al Aoula Inter HD", "1080p FHD")
            )
        )
    )

    fun fetchChannels(group: String = "VIP", search: String = ""): List<LiveChannelItem> {
        val qGroup = URLEncoder.encode(group, "UTF-8")
        val qSearch = URLEncoder.encode(search, "UTF-8")
        val engineUrl = "http://127.0.0.1:${JewEngineManager.ENGINE_PORT}/live/channels?group=$qGroup&search=$qSearch"

        val jsonStr = httpGetRaw(engineUrl, 8000)
        if (!jsonStr.isNullOrEmpty()) {
            try {
                val root = JSONObject(jsonStr)
                val arr = root.optJSONArray("channels") ?: JSONArray()
                val out = mutableListOf<LiveChannelItem>()
                for (i in 0 until arr.length()) {
                    val o = arr.optJSONObject(i) ?: continue
                    val bArr = o.optJSONArray("backups") ?: JSONArray()
                    val backups = mutableListOf<LiveFeedItem>()
                    for (j in 0 until bArr.length()) {
                        val b = bArr.optJSONObject(j) ?: continue
                        backups.add(
                            LiveFeedItem(
                                url = b.optString("url"),
                                source = b.optString("source", "Live Feed"),
                                quality = b.optString("quality", "1080p FHD"),
                                protocol = b.optString("protocol", "HLS")
                            )
                        )
                    }
                    val name = o.optString("name", "Channel")
                    val logo = resolveLogo(name, o.optString("logo", ""))
                    out.add(
                        LiveChannelItem(
                            id = o.optString("id", "ch_$i"),
                            name = name,
                            logo = logo,
                            group = o.optString("group", "Live TV"),
                            quality = o.optString("quality", "1080p FHD"),
                            source = o.optString("source", "Live Headend"),
                            url = o.optString("url"),
                            verified = o.optBoolean("verified", false),
                            backups = backups
                        )
                    )
                }
                if (out.isNotEmpty()) return out
            } catch (_: Exception) {}
        }

        // Instant fallback to built-in VIP channels if engine is still booting
        val q = search.trim().lowercase(Locale.US)
        return getBuiltInVipChannels().filter { ch ->
            val groupMatch = group == "VIP" || group == "ALL" || ch.group.equals(group, ignoreCase = true)
            val searchMatch = q.isEmpty() || ch.name.lowercase(Locale.US).contains(q) || ch.group.lowercase(Locale.US).contains(q)
            groupMatch && searchMatch
        }
    }

    fun toStreamItems(channel: LiveChannelItem): List<StreamItem> {
        val feeds = if (channel.backups.isNotEmpty()) {
            channel.backups
        } else {
            listOf(LiveFeedItem(channel.url, channel.source, channel.quality))
        }
        return feeds.mapIndexed { idx, feed ->
            StreamItem(
                name = "LIVE FEED #${idx + 1}",
                title = "${channel.name} — ${feed.source}",
                url = feed.url,
                quality = feed.quality,
                seeders = 99999 - idx,
                sizeText = "LIVE",
                releaseTitle = "Feed ${idx + 1}: ${feed.source}",
                codecTags = "Live Broadcast • ${feed.quality} • Instant Failover",
                hasArabicHint = channel.group.contains("Arabic", true) || channel.name.contains("Arabic", true),
                isDirect = true
            )
        }
    }

    private fun httpGetRaw(urlString: String, timeoutMs: Int): String? {
        var conn: HttpURLConnection? = null
        return try {
            conn = (URL(urlString).openConnection() as HttpURLConnection).apply {
                connectTimeout = timeoutMs
                readTimeout = timeoutMs
                instanceFollowRedirects = true
                setRequestProperty("User-Agent", "NOIR-TV/3.0")
            }
            if (conn.responseCode in 200..299) {
                BufferedReader(InputStreamReader(conn.inputStream)).use { it.readText() }
            } else null
        } catch (_: Exception) {
            null
        } finally {
            conn?.disconnect()
        }
    }
}
