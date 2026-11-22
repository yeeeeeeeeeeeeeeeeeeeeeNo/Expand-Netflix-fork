package com.d4cx.streamflix

import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.os.Handler
import android.os.Looper
import android.util.LruCache
import android.widget.ImageView
import java.io.ByteArrayOutputStream
import java.io.File
import java.io.FileOutputStream
import java.net.HttpURLConnection
import java.net.URL
import java.security.MessageDigest
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.Executors

/**
 * Low-RAM RGB_565 Image Pipeline tuned for 1.78 GB ARMv7 Android TV.
 * - Lean 16 MB bitmap LRU + 24 MB disk cache
 * - In-flight request deduplication (prevents duplicate network fetches while fast-scrolling D-pad!)
 * - Pure RGB_565 decode (50% lower VRAM/RAM vs ARGB_8888)
 */
object ImageLoader {

    private val mainHandler = Handler(Looper.getMainLooper())
    private val executor = Executors.newFixedThreadPool(6)
    private val inFlight = ConcurrentHashMap.newKeySet<String>()

    private const val DISK_CACHE_MAX_BYTES = 24L * 1024L * 1024L
    @Volatile
    private var diskCacheDir: File? = null

    private val maxMemory = (Runtime.getRuntime().maxMemory() / 1024).toInt()
    private val cacheSize = (maxMemory / 10).coerceIn(8192, 16384)

    private val memoryCache = object : LruCache<String, Bitmap>(cacheSize) {
        override fun sizeOf(key: String, bitmap: Bitmap): Int = bitmap.byteCount / 1024
    }

    fun init(context: Context) {
        val dir = File(context.applicationContext.cacheDir, "img_cache")
        if (!dir.exists()) dir.mkdirs()
        diskCacheDir = dir
        executor.execute { trimDiskCache() }
    }

    fun trimMemory() {
        runCatching { memoryCache.trimToSize(cacheSize / 2) }
    }

    fun load(
        url: String?,
        imageView: ImageView,
        reqWidth: Int = 300,
        reqHeight: Int = 450,
        keepPreviousWhileLoading: Boolean = false
    ) {
        if (url.isNullOrEmpty()) {
            imageView.tag = null
            imageView.setImageDrawable(null)
            return
        }

        imageView.tag = url
        val memKey = "${reqWidth}x${reqHeight}_$url"
        val cached = memoryCache.get(memKey)
        if (cached != null && !cached.isRecycled) {
            imageView.animate().cancel()
            imageView.alpha = 1f
            imageView.setImageBitmap(cached)
            return
        }

        if (!keepPreviousWhileLoading) {
            imageView.setImageDrawable(null)
        }

        executor.execute {
            try {
                val bitmap = loadFromDiskOrNetwork(url, reqWidth, reqHeight)
                if (bitmap != null) {
                    memoryCache.put(memKey, bitmap)
                    mainHandler.post {
                        if (imageView.tag == url) {
                            if (keepPreviousWhileLoading) {
                                imageView.alpha = 0.72f
                                imageView.setImageBitmap(bitmap)
                                imageView.animate().alpha(1f).setDuration(150).start()
                            } else {
                                imageView.alpha = 0f
                                imageView.setImageBitmap(bitmap)
                                imageView.animate().alpha(1f).setDuration(110).start()
                            }
                        }
                    }
                }
            } catch (_: Exception) {}
        }
    }

    fun preload(url: String?, reqWidth: Int = 800, reqHeight: Int = 450) {
        if (url.isNullOrEmpty()) return
        val memKey = "${reqWidth}x${reqHeight}_$url"
        if (memoryCache.get(memKey) != null || !inFlight.add(memKey)) return
        executor.execute {
            try {
                loadFromDiskOrNetwork(url, reqWidth, reqHeight)?.let { bmp ->
                    memoryCache.put(memKey, bmp)
                }
            } catch (_: Exception) {
            } finally {
                inFlight.remove(memKey)
            }
        }
    }

    private fun loadFromDiskOrNetwork(urlString: String, reqWidth: Int, reqHeight: Int): Bitmap? {
        val diskFile = cacheFileFor(urlString)
        if (diskFile != null && diskFile.exists() && diskFile.length() > 128) {
            try {
                diskFile.setLastModified(System.currentTimeMillis())
                val bytes = diskFile.readBytes()
                decodeSampled(bytes, reqWidth, reqHeight)?.let { return it }
            } catch (_: Exception) {}
        }

        var connection: HttpURLConnection? = null
        return try {
            connection = (URL(urlString).openConnection() as HttpURLConnection).apply {
                connectTimeout = 3800
                readTimeout = 4200
                instanceFollowRedirects = true
                useCaches = true
                setRequestProperty("User-Agent", "NOIR-TV/2.0")
            }
            if (connection.responseCode !in 200..299) return null

            val bytes = connection.inputStream.use { input ->
                val out = ByteArrayOutputStream()
                val buffer = ByteArray(8192)
                while (true) {
                    val n = input.read(buffer)
                    if (n < 0) break
                    if (out.size() + n > 5 * 1024 * 1024) return null
                    out.write(buffer, 0, n)
                }
                out.toByteArray()
            }
            if (bytes.isEmpty()) return null

            if (diskFile != null && bytes.size < 2 * 1024 * 1024) {
                try {
                    synchronized(this) {
                        val temp = File(diskFile.parentFile, diskFile.name + ".tmp")
                        FileOutputStream(temp).use { it.write(bytes) }
                        if (!temp.renameTo(diskFile)) temp.delete()
                        trimDiskCache()
                    }
                } catch (_: Exception) {}
            }

            decodeSampled(bytes, reqWidth, reqHeight)
        } catch (_: Exception) {
            null
        } finally {
            connection?.disconnect()
        }
    }

    private fun decodeSampled(bytes: ByteArray, reqWidth: Int, reqHeight: Int): Bitmap? {
        val options = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        BitmapFactory.decodeByteArray(bytes, 0, bytes.size, options)
        if (options.outWidth <= 0 || options.outHeight <= 0) return null

        options.inSampleSize = calculateInSampleSize(options, reqWidth, reqHeight)
        options.inJustDecodeBounds = false
        options.inPreferredConfig = Bitmap.Config.RGB_565
        return BitmapFactory.decodeByteArray(bytes, 0, bytes.size, options)
    }

    private fun calculateInSampleSize(options: BitmapFactory.Options, reqWidth: Int, reqHeight: Int): Int {
        val height = options.outHeight
        val width = options.outWidth
        var inSampleSize = 1
        if (height > reqHeight || width > reqWidth) {
            val halfHeight = height / 2
            val halfWidth = width / 2
            while (halfHeight / inSampleSize >= reqHeight && halfWidth / inSampleSize >= reqWidth) {
                inSampleSize *= 2
            }
        }
        return inSampleSize.coerceAtLeast(1)
    }

    private fun cacheFileFor(url: String): File? {
        val dir = diskCacheDir ?: return null
        val digest = MessageDigest.getInstance("SHA-256").digest(url.toByteArray())
        val hex = digest.joinToString("") { "%02x".format(it) }.take(32)
        return File(dir, "$hex.img")
    }

    private fun trimDiskCache() {
        val dir = diskCacheDir ?: return
        val files = dir.listFiles()?.toList() ?: return
        var total = files.sumOf { it.length() }
        if (total <= DISK_CACHE_MAX_BYTES) return
        val sortedOldestFirst = files.sortedBy { it.lastModified() }
        for (f in sortedOldestFirst) {
            if (total <= DISK_CACHE_MAX_BYTES * 3 / 4) break
            val len = f.length()
            if (f.delete()) total -= len
        }
    }
}
