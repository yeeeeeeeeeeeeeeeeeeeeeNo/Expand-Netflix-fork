package com.d4cx.streamflix

import android.util.Log
import org.json.JSONObject
import java.io.BufferedReader
import java.io.InputStreamReader
import java.net.HttpURLConnection
import java.net.URL
import java.nio.ByteBuffer
import java.nio.charset.Charset
import java.nio.charset.CodingErrorAction
import java.util.Locale

data class SubtitleItem(
    val id: String,
    val langCode: String,
    val langLabel: String,
    val url: String
)

object SubtitleRepository {
    private const val TAG = "SubtitleRepository"
    private val SRT_TS_REGEX = Regex("""(\d{2}):(\d{2}):(\d{2})([,\.])(\d{3})""")

    private val languageNames = mapOf(
        "eng" to "English",
        "ara" to "Arabic (العربية)",
        "jpn" to "Japanese (日本語)",
        "spa" to "Spanish (Español)",
        "fre" to "French (Français)",
        "fra" to "French (Français)",
        "ger" to "German (Deutsch)",
        "deu" to "German (Deutsch)",
        "ita" to "Italian (Italiano)",
        "por" to "Portuguese (Português)",
        "pob" to "Portuguese (BR)",
        "rus" to "Russian (Русский)",
        "chi" to "Chinese (中文)",
        "zho" to "Chinese (中文)",
        "kor" to "Korean (한국어)",
        "tur" to "Turkish (Türkçe)",
        "pol" to "Polish (Polski)",
        "dut" to "Dutch (Nederlands)",
        "nld" to "Dutch (Nederlands)",
        "ind" to "Indonesian",
        "hin" to "Hindi",
        "swe" to "Swedish",
        "nor" to "Norwegian",
        "dan" to "Danish",
        "fin" to "Finnish",
        "gre" to "Greek",
        "ell" to "Greek",
        "heb" to "Hebrew",
        "hun" to "Hungarian",
        "ces" to "Czech",
        "cze" to "Czech",
        "ron" to "Romanian",
        "rum" to "Romanian",
        "tha" to "Thai",
        "vie" to "Vietnamese"
    )

    fun fetchSubtitles(type: String, id: String, season: Int? = null, episode: Int? = null): List<SubtitleItem> {
        val streamType = if (type == "anime") "series" else type
        val queryId = if (streamType == "series") {
            if (id.contains(":")) id
            else if (season != null && episode != null) "$id:$season:$episode"
            else "$id:1:1"
        } else {
            id
        }

        val urlStr = "https://opensubtitles-v3.strem.io/subtitles/$streamType/$queryId.json"
        val jsonStr = httpGet(urlStr, 4200) ?: return emptyList()

        val results = mutableListOf<SubtitleItem>()
        try {
            val root = JSONObject(jsonStr)
            val subsArr = root.optJSONArray("subtitles") ?: return emptyList()
            val langCounts = mutableMapOf<String, Int>()
            val seenUrls = mutableSetOf<String>()

            for (i in 0 until subsArr.length()) {
                val obj = subsArr.getJSONObject(i)
                val subId = obj.optString("id", "$i")
                val lang = obj.optString("lang", "eng").lowercase()
                val url = obj.optString("url", "")
                if (url.isNotEmpty() && !seenUrls.contains(url)) {
                    val count = (langCounts[lang] ?: 0) + 1
                    if (count <= 4) {
                        seenUrls.add(url)
                        langCounts[lang] = count
                        val baseName = languageNames[lang] ?: lang.uppercase()
                        val displayName = if (count == 1) baseName else "$baseName #$count"
                        results.add(SubtitleItem(subId, lang, displayName, url))
                    }
                }
            }
        } catch (e: Exception) {
            Log.e(TAG, "Failed to parse subtitles: ${e.message}")
        }

        return results.sortedWith(compareBy({
            when (it.langCode) {
                "ara" -> 0
                "eng" -> 1
                "jpn" -> 2
                "fre", "fra" -> 3
                "spa" -> 4
                else -> 8
            }
        }, { it.langLabel }))
    }

    /**
     * Auto-repairs Arabic Windows-1256 / ISO-8859-6 subtitle files into clean UTF-8
     * so Arabic subtitles NEVER show garbled mojibake on Android TV!
     */
    fun decodeSubtitleBytesToUtf8(raw: ByteArray, langCode: String): String {
        val utf8Decoder = Charsets.UTF_8.newDecoder()
            .onMalformedInput(CodingErrorAction.REPORT)
            .onUnmappableCharacter(CodingErrorAction.REPORT)
        val strictUtf8 = runCatching { utf8Decoder.decode(ByteBuffer.wrap(raw)).toString() }.getOrNull()
        if (strictUtf8 != null && !strictUtf8.contains('\uFFFD')) {
            return strictUtf8.removePrefix("\uFEFF")
        }
        val fallbackCharset = if (langCode.startsWith("ar", ignoreCase = true)) {
            runCatching { Charset.forName("windows-1256") }.getOrDefault(Charsets.UTF_8)
        } else {
            runCatching { Charset.forName("windows-1252") }.getOrDefault(Charsets.UTF_8)
        }
        return String(raw, fallbackCharset).removePrefix("\uFEFF")
    }

    /** Shifts every SRT / WebVTT timestamp by offsetMs for live in-player subtitle sync! */
    fun shiftSubtitleTimestamps(content: String, offsetMs: Long): String {
        if (offsetMs == 0L) return content
        return SRT_TS_REGEX.replace(content) { m ->
            val h = m.groupValues[1].toLongOrNull() ?: 0L
            val min = m.groupValues[2].toLongOrNull() ?: 0L
            val sec = m.groupValues[3].toLongOrNull() ?: 0L
            val sep = m.groupValues[4]
            val ms = m.groupValues[5].toLongOrNull() ?: 0L
            val total = (h * 3600_000L + min * 60_000L + sec * 1000L + ms + offsetMs).coerceAtLeast(0L)
            val nh = total / 3600_000L
            val nmin = (total / 60_000L) % 60L
            val nsec = (total / 1000L) % 60L
            val nms = total % 1000L
            String.format(Locale.US, "%02d:%02d:%02d%s%03d", nh, nmin, nsec, sep, nms)
        }
    }

    private fun httpGet(urlString: String, timeoutMs: Int = 4200): String? {
        var conn: HttpURLConnection? = null
        return try {
            conn = (URL(urlString).openConnection() as HttpURLConnection).apply {
                connectTimeout = timeoutMs
                readTimeout = timeoutMs
                instanceFollowRedirects = true
                setRequestProperty("User-Agent", "NOIR-TV/2.0 (Android TV)")
                setRequestProperty("Accept", "application/json")
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
