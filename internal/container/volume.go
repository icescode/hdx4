package container

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hardix/pkg/spec"
	"io"
)

// TrackEntry disesuaikan dengan struktur HDX3
type TrackEntry struct {
	TrackNumber int     `json:"track_number"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	OriginFile  string  `json:"origin_file"`
	Offset      uint64  `json:"offset"`
	Size        uint64  `json:"size"`
	Duration    float64 `json:"duration"`
	Fingerprint string  `json:"fingerprint"`
}

type ReadSeekerAt interface {
	io.ReadSeeker
	io.ReaderAt
}

type Volume struct {
	Reader     ReadSeekerAt
	AlbumTitle string
	Publisher  string
	Tracks     []TrackEntry
}

func UnpackVolume(r ReadSeekerAt) (*Volume, error) {
	// 1. Validasi Magic Number
	magic := make([]byte, 8) // Mengikuti HARDIX02 (8 byte)
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, fmt.Errorf("failed to read magic: %v", err)
	}

	if string(magic) != spec.MagicV2 && string(magic)[:6] != spec.VolumeMagicV2 {
		return nil, fmt.Errorf("invalid volume magic: %s", string(magic))
	}

	vol := &Volume{Reader: r}

	// 2. Loop pembacaan Tag
	for {
		tagBuf := make([]byte, 4)
		if _, err := io.ReadFull(r, tagBuf); err != nil {
			if err == io.EOF {
				break // Keluar loop jika file habis
			}
			return nil, err
		}
		tag := string(tagBuf)

		var size uint32
		if err := binary.Read(r, binary.BigEndian, &size); err != nil {
			return nil, err
		}

		switch tag {
		case spec.Album: // "ALBM"
			buf := make([]byte, size)
			io.ReadFull(r, buf)
			vol.AlbumTitle = string(buf)

		case spec.Publisher: // "PUBL"
			buf := make([]byte, size)
			io.ReadFull(r, buf)
			vol.Publisher = string(buf)

		case "TTOC":
			buf := make([]byte, size)
			io.ReadFull(r, buf)
			var tracks []TrackEntry
			if err := json.Unmarshal(buf, &tracks); err != nil {
				return nil, fmt.Errorf("failed to parse TTOC: %v", err)
			}
			vol.Tracks = tracks
			// Kita tidak langsung return di sini agar bisa membaca tag lain jika ada
			// namun biasanya TTOC diletakkan di akhir oleh volmaker.

		default:
			// Loncat ke tag berikutnya
			if _, err := r.Seek(int64(size), io.SeekCurrent); err != nil {
				return nil, err
			}
		}
	}

	// Pastikan minimal ada track yang ditemukan
	if len(vol.Tracks) == 0 {
		return nil, fmt.Errorf("no tracks found in volume (TTOC missing or empty)")
	}

	return vol, nil
}
