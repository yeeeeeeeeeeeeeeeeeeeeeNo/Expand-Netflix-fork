package com.d4cx.streamflix

import android.util.Log
import java.io.BufferedReader
import java.io.InputStreamReader
import java.net.Inet4Address
import java.net.InetSocketAddress
import java.net.NetworkInterface
import java.net.ServerSocket
import java.net.Socket
import java.net.URLDecoder
import java.util.concurrent.atomic.AtomicBoolean

/**
 * Zero-dependency LAN Phone Keyboard Server (TCP :8812).
 * Open http://<tv-ip>:8812 on your phone browser to type movie/series searches straight onto NOIR TV!
 */
class LanSearchServer(
    private val onRemoteQuery: (String) -> Unit
) {
    companion object {
        const val PORT = 8812
        private const val TAG = "NoirLanSearch"
    }

    private val running = AtomicBoolean(false)
    private var serverSocket: ServerSocket? = null

    fun start() {
        if (!running.compareAndSet(false, true)) return
        Thread({
            try {
                val ss = ServerSocket().apply {
                    reuseAddress = true
                    bind(InetSocketAddress(PORT))
                }
                serverSocket = ss
                while (running.get()) {
                    val client = ss.accept()
                    Thread({ handleClient(client) }, "NoirLanClient").apply { isDaemon = true }.start()
                }
            } catch (e: Exception) {
                Log.d(TAG, "LAN server stopped: ${e.message}")
            }
        }, "NoirLanSearch").apply { isDaemon = true }.start()
    }

    fun stop() {
        running.set(false)
        runCatching { serverSocket?.close() }
        serverSocket = null
    }

    fun getDisplayAddress(): String {
        val ip = getLocalIpv4() ?: "tv-ip"
        return "http://$ip:$PORT"
    }

    private fun handleClient(sock: Socket) {
        runCatching {
            sock.use { s ->
                s.soTimeout = 3000
                val br = BufferedReader(InputStreamReader(s.getInputStream(), Charsets.UTF_8))
                val reqLine = br.readLine() ?: return@use
                val path = reqLine.split(" ").getOrNull(1) ?: "/"
                val os = s.getOutputStream()

                if (path.startsWith("/q?")) {
                    val raw = path.substringAfter("text=", "").substringBefore("&")
                    val decoded = runCatching { URLDecoder.decode(raw, "UTF-8") }.getOrDefault("").trim()
                    if (decoded.isNotEmpty()) onRemoteQuery(decoded)
                    val body = "{\"ok\":true}"
                    val resp = "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: ${body.length}\r\nConnection: close\r\n\r\n$body"
                    os.write(resp.toByteArray(Charsets.UTF_8))
                    os.flush()
                    return@use
                }

                val html = """
                    <!DOCTYPE html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
                    <title>NOIR TV Remote Keyboard</title>
                    <style>
                    body{margin:0;background:#000;color:#fff;font-family:-apple-system,sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;padding:20px;box-sizing:border-box}
                    .box{width:100%;max-width:420px;border:1px solid #2a2a30;border-radius:10px;padding:24px;background:#0a0a0c}
                    h1{margin:0 0 6px;font-size:22px;letter-spacing:.18em}
                    p{margin:0 0 18px;color:#8e929e;font-size:13px}
                    input{width:100%;padding:14px;font-size:16px;background:#141418;color:#fff;border:1px solid #353540;border-radius:6px;box-sizing:border-box;outline:none}
                    input:focus{border-color:#fff}
                    button{width:100%;margin-top:12px;padding:14px;font-size:15px;font-weight:800;background:#fff;color:#000;border:none;border-radius:6px;cursor:pointer}
                    #st{margin-top:12px;font-size:13px;color:#b8bcc6;min-height:18px}
                    </style></head><body><div class="box"><h1>NOIR</h1><p>Type to search instantly on your TV</p>
                    <form onsubmit="go(event)"><input id="q" placeholder="Movie, series, anime or Arabic..." autofocus>
                    <button type="submit">Send to TV</button></form><div id="st"></div></div>
                    <script>function go(e){e.preventDefault();var v=document.getElementById('q').value.trim();if(!v)return;
                    fetch('/q?text='+encodeURIComponent(v)).then(function(){document.getElementById('st').textContent='Sent: '+v;});}</script></body></html>
                """.trimIndent()
                val bytes = html.toByteArray(Charsets.UTF_8)
                val header = "HTTP/1.1 200 OK\r\nContent-Type: text/html; charset=utf-8\r\nContent-Length: ${bytes.size}\r\nConnection: close\r\n\r\n"
                os.write(header.toByteArray(Charsets.US_ASCII))
                os.write(bytes)
                os.flush()
            }
        }
    }

    private fun getLocalIpv4(): String? {
        return runCatching {
            val ifaces = NetworkInterface.getNetworkInterfaces() ?: return null
            for (ni in ifaces) {
                if (!ni.isUp || ni.isLoopback) continue
                for (addr in ni.inetAddresses) {
                    if (addr is Inet4Address && !addr.isLoopbackAddress) {
                        return addr.hostAddress
                    }
                }
            }
            null
        }.getOrNull()
    }
}
