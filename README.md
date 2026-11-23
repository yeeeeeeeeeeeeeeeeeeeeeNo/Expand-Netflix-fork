# NOCTURNE v3.2 — Cinema & Live TV for Android TV & Windows PC

A high-performance streaming and live broadcast application engineered for **Android TV** (Xiaomi Mi TV, Chromecast, Android 9-14) and **Windows PC** (standalone native executable with GPU acceleration). Features unified catalog discovery, multi-addon torrent streaming, real-time IPTV channel decoding, audio stream switching, and WebGL2 super-resolution upscaling.

---

## What's New in v3.2 (TV & PC Release)

### 1. Live TV Channels Engine (`/tv` & `/live`)
- **Built-in Global & Regional Live TV**: Integrated live channel portal categorized into Cinema, Sports, News, Anime & Entertainment, and Arabic suites (BeIN, MBC, Rotana, OSN, and more).
- **Zero-Stall Stream Engine**: Go-powered HTTP live stream reverse proxy with dynamic header spoofing (User-Agent/Referer bypass) and low-latency chunk buffering.
- **TV Gate Navigator**: Smooth Android TV D-Pad switch between the on-demand Cinema catalog and the full-screen Live TV Channels grid.

### 2. Standalone Windows PC Port (`NOCTURNE.exe`)
- **Native Frameless Win32 WebView2 Shell**: Ultra-fast standalone PC executable compiled directly from Go. No browser tabs, no Electron bloat, zero background telemetry.
- **True Borderless Fullscreen**: Toggle full desktop display with `F`, `F11`, or double-clicking the cinema canvas.
- **Minimalist Cinema Player HUD**: Custom sleek controls with auto-hiding interface, scrub timeline, Skip Intro (`+85s`), volume slider, and responsive track controls.
- **Audio Language Switcher**: Real-time multi-audio track parsing with English preference auto-routing and instant live language switching preserving current seek position.
- **Accurate Runtime Duration**: Precise FFmpeg container duration probe resolving dynamic MP4 timelines.
- **Multi-Category Relevance Search**: Intelligent ranking across Movies, Series, and Anime with strict poster cover validation.

### 3. WebGL2 GPU Upscaling Engine
- **FSR + RCAS Super-Resolution**: High-performance spatial upscaling with contrast-adaptive sharpening running on your PC GPU via WebGL2 shaders.
- **Anime4K Neural Pass**: Specialized cartoon and anime edge reconstruction filter preserving sharpness without color or saturation distortion.
- **Adaptive Network Mode**: "Meh-Wi-Fi" buffering profile optimizing chunk concurrency and socket timeouts for non-gigabit connections.

---

## Downloads (Releases)

Grab pre-compiled release artifacts directly from the [GitHub Releases](https://github.com/yeeeeeeeeeeeeeeeeeeeeeNo/Expand-Netflix-fork/releases):
- **Windows PC**: `NOCTURNE-PC-Portable.zip` (standalone `NOCTURNE.exe` + bundled `ffmpeg.exe`)
- **Android TV**: `NOCTURNE-Android-TV.apk` (sideload via ADB or USB flash drive)

---

## Build Instructions

### Android TV APK
Requires JDK 17 (`JAVA_HOME`) and Android SDK Platform 34 (`ANDROID_HOME`):
```powershell
.\gradlew.bat assembleDebug
# Or for release:
.\gradlew.bat assembleRelease
```
Output: `app/build/outputs/apk/debug/app-debug.apk`

### Windows PC Executable
Requires Go 1.22+ and GCC (MinGW-w64):
```powershell
cd engine
go build -ldflags "-H windowsgui -s -w" -o NOCTURNE.exe .
```
Package with `ffmpeg.exe` for real-time audio and transcode probing.
