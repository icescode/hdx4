package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"

	"hardix/internal/security"
	"hardix/pkg/spec"
)

type TrackEntry struct {
	TrackNumber int     `json:"track_number"`
	Title       string  `json:"title"`
	Offset      uint64  `json:"offset"`
	Duration    float64 `json:"duration"`
}

type VolumeMeta struct {
	Album   string       `json:"album"`
	Artist  string       `json:"artist"`
	Content []TrackEntry `json:"content"`
}

type VolumeRuntime struct {
	Path     string
	Meta     VolumeMeta
	AudioKey []byte
}

var volumes []VolumeRuntime

// === DISIPLIN: SATU CARA LOAD VOLUME ===
func loadVolumes() {
	cfg := os.Getenv("HDX_VOLUME_LIST")
	if cfg == "" {
		cfg = os.Getenv("HOME") + "/.hdx-socket_list"
	}

	data, err := os.ReadFile(cfg)
	if err != nil {
		panic("cannot read volume list")
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")

	for _, path := range lines {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}

		dat := strings.TrimSuffix(path, ".hdxv") + "_keys.dat"

		pass, err := security.UnlockKeyLocker(path, dat, "")
		if err != nil {
			continue
		}
		key := security.DeriveKey(pass, []byte(spec.Salt))

		f, _ := os.Open(path)
		stat, _ := f.Stat()
		f.Seek(stat.Size()-int64(10*1024*1024), io.SeekStart)
		raw, _ := io.ReadAll(f)
		f.Close()

		idx := strings.LastIndex(string(raw), "JSFD")
		if idx < 0 {
			continue
		}

		var meta VolumeMeta
		json.Unmarshal(raw[idx+8:], &meta)

		volumes = append(volumes, VolumeRuntime{
			Path:     path,
			Meta:     meta,
			AudioKey: key,
		})
	}

	if len(volumes) == 0 {
		panic("no valid HDX volumes loaded")
	}
}
