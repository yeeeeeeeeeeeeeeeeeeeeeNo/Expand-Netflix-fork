# NOIR v2.0 — Monochrome Cinema for Android TV

A stark **black-and-white Netflix-style Android TV streaming app** built with native Android Views, Media3 ExoPlayer, and an embedded rolling-cache Go torrent engine. Engineered for low-RAM Android TV hardware (tested on Xiaomi Mi TV `MiTV-MZTU0`, ARMv7, 1.78 GB RAM, Android 14).

## What's New in NOIR v2.0
- **Complete Rebrand & Monochrome Netflix TV UI Overhaul**:
  - **Cinematic First-Boot Intro**: Pitch-black aperture beam reveal into the monumental `NOIR` wordmark while the catalog & native Go engine warm up in parallel underneath.
  - **Floating Netflix Top Navigation Bar**: Replaces the cramped left sidebar with an effortless top nav (`Home · Search · Browse & Arabic · My List · Settings`) — press `UP` from the billboard to switch tabs, `DOWN` to browse.
  - **Full-Bleed Live Hero Billboard**: Moving across any movie, series, Continue Watching, or Arabic card dynamically cross-fades the full-bleed widescreen backdrop, title lockup, metadata badges, synopsis, and Netflix-style `▶ Play` / `More Info` / `+ My List` buttons.
  - **Crisp Monochrome Cards**: Pure white 3dp TV focus rings, 60fps optical scale lift, and stark white progress bars.
- **Parallel Multi-Addon Stream Racer (`StreamRepository`)**:
  - Simultaneously races your configured primary addon alongside **Comet** and **MediaFusion** (plus Torrentio & Kitsu fallbacks) with a 90-second memory cache — cutting dead-source stalls and speeding up start times dramatically.
  - Re-enabled direct **Arabic Dub Archive** & **StarDima** live Arabic streams.
- **Cinema Player Upgrades (`PlayerActivity`)**:
  - **Monochrome `SKIP INTRO ▸` Pill**: Appears cleanly during series/anime openings for 1-click intro skipping.
  - **Background Next-Episode Pre-Warm**: Quietly fetches Episode `N+1` streams during the last 90 seconds so binge-watching starts instantly.
  - **Arabic `Windows-1256` → `UTF-8` Subtitle Repair + Live Sync (`-0.5s / +0.5s`)**: Eliminates garbled Arabic subtitles and lets you shift subtitle timing live inside the player.
- **Phone LAN Remote Keyboard (`LanSearchServer` on TCP `:8812`)**:
  - Open `http://<tv-ip>:8812` on your phone browser to type searches straight into NOIR TV without slow D-pad typing.
- **Low-RAM Xiaomi TV Optimizations**:
  - Lean 16 MB `RGB_565` image cache with in-flight deduplication and `onTrimMemory` eviction.
  - R8 release minification & resource shrinking (`minifyEnabled true`, `shrinkResources true`).

## Build

Requires JDK 17 and Android SDK Platform 34:

```powershell
.\gradlew.bat assembleRelease
```

APK output: `app/build/outputs/apk/release/app-release.apk`.
