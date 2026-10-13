# Lyric Studio (non-AI, Windows)

An offline Go + Fyne desktop starter app for lyric videos. It uses FFmpeg/ffprobe and ordinary audio filters; it does not use AI, speech recognition, cloud services, or a Python runtime.

## Features

- Import audio: MP3, WAV, M4A, FLAC, AAC, OGG, OPUS, WMA (actual support depends on your FFmpeg build).
- Import `.lrc` timestamp files. LRC timestamps are converted into lyric events.
- Paste untimed lyrics: one lyric line per row, distributed evenly over the song. This is convenient but not word-perfect synchronization.
- For precise manual timing, use one row per lyric in this format: `00:12.450 | lyric text` (MM:SS.mmm).
- Select `Cinematic` or `Audio Spectrum` visuals.
- Export 1080p or 4K at 60 FPS, H.264 MP4 with AAC audio.
- All processing is local.

## Windows setup

### 1. Install Go

Install Go 1.23 or newer from https://go.dev/dl/ and reopen PowerShell. Check:

```powershell
go version
```

### 2. Install FFmpeg

```powershell
winget install --id Gyan.FFmpeg -e
```
Close and reopen PowerShell, then check both commands:

```powershell
ffmpeg -version
ffprobe -version
```

If either command is not found, add the FFmpeg `bin` folder to PATH.

### 3. Install a C compiler for Fyne

Fyne's desktop driver on Windows requires CGO. Install a MinGW-w64 toolchain such as WinLibs (UCRT, x86_64) and add its `mingw64\bin` folder to PATH. Check:

```powershell
gcc --version
go env CGO_ENABLED
```

Set CGO on for the current PowerShell session if needed:

```powershell
$env:CGO_ENABLED = "1"
```

### 4. Build and run

Extract this folder, open PowerShell in the project directory, then run:

```powershell
go mod tidy
go run .\cmd\lyricstudio
```

Build a Windows executable:

```powershell
go build -o .\LyricStudio.exe .\cmd\lyricstudio
```

## LRC format

Standard LRC example:

```text
[00:02.50]First line of the song
[00:06.20]Second line of the song
[00:10.00]Third line of the song
```

The app accepts centisecond or millisecond-style fractions in timestamps. Lines without timestamps and metadata tags such as `[ti:Title]` are ignored.

## Manual timing format

```text
00:02.500 | First line of the song
00:06.200 | Second line of the song
00:10.000 | Third line of the song
```

Use either all timestamped rows or all plain rows. Plain rows are evenly distributed across the audio duration. For good synchronization, prefer LRC or manually timestamped rows.

## Rendering notes

- The cinematic preset uses a dark textured background, an audio-reactive waveform, and styled lyric subtitles.
- The Audio Spectrum preset uses FFmpeg's `showspectrum` filter as the moving background, with lyric subtitles over it.
- 4K/60 FPS with the `slow` x264 preset is compute-intensive and can take substantially longer than the song duration. Use 1080p first to test timing and filter support.
- The render uses CPU-based `libx264` for broad compatibility and quality. NVIDIA GPU is not required because this version does not use AI. Hardware encoding can be added as an optional speed mode, but quality/supported codecs vary by GPU and driver.
- Keep FFmpeg updated. If a render fails, the app's Render Log includes FFmpeg's error output.

## Current scope / limitations

This is a starter editor, not a full nonlinear video editor. It has no waveform timeline drag handles yet. Untimed lyrics are evenly spaced rather than automatically aligned to sung words. Audio-reactive visuals are produced by FFmpeg filters and traditional signal processing, not machine learning.
