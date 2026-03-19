package common

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/abema/go-mp4"
	mpeg2 "github.com/yapingcat/gomedia/go-mpeg2"
)

// GetVideoDuration returns the video duration in seconds and whether the video contains an audio track.
// It uses pure Go parsing and does not rely on external binaries like ffprobe.
//
// Supported MIME types:
//   - video/mp4
//   - video/mpeg (best-effort: detects TS/PS and scans PTS)
func GetVideoDuration(ctx context.Context, f io.ReadSeeker, mimeType string) (durationSeconds float64, hasAudio bool, err error) {
	_ = ctx
	switch mimeType {
	case "video/mp4":
		return getMP4DurationAndAudio(f)
	case "video/mpeg":
		return getMPEGDurationAndAudio(f)
	default:
		return 0, false, fmt.Errorf("unsupported video mime type: %s", mimeType)
	}
}

func getMP4DurationAndAudio(r io.ReadSeeker) (float64, bool, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return 0, false, fmt.Errorf("failed to seek mp4: %w", err)
	}
	info, err := mp4.Probe(r)
	if err != nil {
		return 0, false, fmt.Errorf("failed to probe mp4: %w", err)
	}
	if info.Timescale == 0 {
		return 0, false, fmt.Errorf("invalid mp4 timescale")
	}

	hasAudio := false
	for _, track := range info.Tracks {
		if track == nil {
			continue
		}
		if track.Codec == mp4.CodecMP4A {
			hasAudio = true
			break
		}
	}

	return float64(info.Duration) / float64(info.Timescale), hasAudio, nil
}

func getMPEGDurationAndAudio(r io.ReadSeeker) (float64, bool, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return 0, false, fmt.Errorf("failed to seek mpeg: %w", err)
	}

	peek := make([]byte, 188*2)
	n, _ := io.ReadFull(r, peek)
	peek = peek[:n]
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return 0, false, fmt.Errorf("failed to seek mpeg: %w", err)
	}

	isTS := len(peek) >= 188*2 && peek[0] == 0x47 && peek[188] == 0x47
	isPS := len(peek) >= 4 && bytes.Equal(peek[:4], []byte{0x00, 0x00, 0x01, 0xBA})

	if isTS {
		return scanMPEGTS(r)
	}
	if isPS {
		return scanMPEGPS(r)
	}

	// Unknown container; try TS then PS as a best-effort fallback.
	if dur, audio, err := scanMPEGTS(r); err == nil {
		return dur, audio, nil
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return 0, false, fmt.Errorf("failed to seek mpeg: %w", err)
	}
	return scanMPEGPS(r)
}

func scanMPEGTS(r io.ReadSeeker) (float64, bool, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return 0, false, fmt.Errorf("failed to seek mpegts: %w", err)
	}

	var (
		hasAudio bool
		ptsSeen  bool
		minPtsMs uint64
		maxPtsMs uint64
	)

	demuxer := mpeg2.NewTSDemuxer()
	demuxer.OnFrame = func(cid mpeg2.TS_STREAM_TYPE, frame []byte, pts uint64, dts uint64) {
		_ = frame
		_ = dts
		switch cid {
		case mpeg2.TS_STREAM_AAC, mpeg2.TS_STREAM_AUDIO_MPEG1, mpeg2.TS_STREAM_AUDIO_MPEG2:
			hasAudio = true
		}
		if !ptsSeen {
			minPtsMs = pts
			maxPtsMs = pts
			ptsSeen = true
			return
		}
		if pts < minPtsMs {
			minPtsMs = pts
		}
		if pts > maxPtsMs {
			maxPtsMs = pts
		}
	}

	if err := demuxer.Input(r); err != nil {
		return 0, false, fmt.Errorf("failed to parse mpegts: %w", err)
	}
	if !ptsSeen {
		return 0, hasAudio, fmt.Errorf("failed to determine mpegts duration: no pts found")
	}

	var durMs uint64
	if maxPtsMs >= minPtsMs {
		durMs = maxPtsMs - minPtsMs
	} else {
		durMs = maxPtsMs
	}
	return float64(durMs) / 1000.0, hasAudio, nil
}

func scanMPEGPS(r io.ReadSeeker) (float64, bool, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return 0, false, fmt.Errorf("failed to seek mpegps: %w", err)
	}

	var (
		hasAudio bool
		ptsSeen  bool
		minPtsMs uint64
		maxPtsMs uint64
	)

	demuxer := mpeg2.NewPSDemuxer()
	demuxer.OnFrame = func(frame []byte, cid mpeg2.PS_STREAM_TYPE, pts uint64, dts uint64) {
		_ = frame
		_ = cid
		_ = dts
		if !ptsSeen {
			minPtsMs = pts
			maxPtsMs = pts
			ptsSeen = true
			return
		}
		if pts < minPtsMs {
			minPtsMs = pts
		}
		if pts > maxPtsMs {
			maxPtsMs = pts
		}
	}
	demuxer.OnPacket = func(pkg mpeg2.Display, decodeResult error) {
		_ = decodeResult
		if pes, ok := pkg.(*mpeg2.PesPacket); ok {
			// PES stream id range: audio 0xC0..0xDF, video 0xE0..0xEF
			if pes.Stream_id >= 0xC0 && pes.Stream_id <= 0xDF {
				hasAudio = true
			}
		}
	}

	buf := make([]byte, 64*1024)
	for {
		n, readErr := r.Read(buf)
		if n > 0 {
			if err := demuxer.Input(buf[:n]); err != nil {
				if e, ok := err.(mpeg2.Error); ok && e.NeedMore() {
					// Need more data; keep scanning.
				} else {
					return 0, hasAudio, fmt.Errorf("failed to parse mpegps: %w", err)
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return 0, hasAudio, fmt.Errorf("failed to read mpegps: %w", readErr)
		}
	}
	demuxer.Flush()

	if !ptsSeen {
		return 0, hasAudio, fmt.Errorf("failed to determine mpegps duration: no pts found")
	}

	var durMs uint64
	if maxPtsMs >= minPtsMs {
		durMs = maxPtsMs - minPtsMs
	} else {
		durMs = maxPtsMs
	}
	return float64(durMs) / 1000.0, hasAudio, nil
}
