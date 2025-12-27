package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"

	"hardix/internal/security"
	"hardix/pkg/audioengine"
	"hardix/pkg/spec"

	"github.com/chzyer/readline"
)

type VolumeStructure struct {
	Album     string       `json:"album"`
	Artist    string       `json:"artist"`
	Publisher string       `json:"publisher"`
	Content   []TrackEntry `json:"content"`
	//edited
	ReleaseDate string `json:"release_date"`
	Genre       string `json:"genre"`
	CopyRight   string `json:"copyright"`
	VolumeInfo  string `json:"volume_info"`
	CreatedDate string `json:"created_date"`
	ArtworkPath string `json:"artwork_path"`
}

type TrackEntry struct {
	TrackNumber int     `json:"track_number"`
	Title       string  `json:"title"`
	OriginFile  string  `json:"origin_file"`
	Offset      uint64  `json:"offset"`
	Size        uint64  `json:"size"`
	Duration    float64 `json:"duration"`
	Fingerprint string  `json:"fingerprint"`
}

const (
	version_minor      = 0
	version_major      = 1
	developer_title    = "Developer Hardiyanto"
	developer_subtitle = "Build 27/12/2025 Ebiet Version"
	app_name           = "HDX-Volmaker"
)

func main() {
	jsonPath, destFolder, password, _ := runVolmakerInterview()

	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		fmt.Printf("[FAIL] Gagal baca JSON: %v\n", err)
		return
	}
	var volStruct VolumeStructure
	json.Unmarshal(jsonData, &volStruct)

	safeAlbum := strings.ReplaceAll(volStruct.Album, " ", "_")
	finalPath := filepath.Join(destFolder, safeAlbum+".hdxv")
	audioKey := security.DeriveKey(password, []byte(spec.Salt))

	f, err := os.Create(finalPath)
	if err != nil {
		fmt.Printf("[FAIL] Gagal buat file: %v\n", err)
		return
	}

	fmt.Printf("\n[START] FORGING: %s\n", volStruct.Album)

	// 1. HEADER & METADATA
	f.Write([]byte(spec.VolumeMagicV2))
	writeTagB(f, spec.Album, []byte(volStruct.Album))

	if volStruct.Artist != "" {
		fmt.Printf(" >> Writing Artist: %s\n", volStruct.Artist)
		writeTagB(f, spec.Genre, []byte(volStruct.Artist))
	}
	if volStruct.Publisher != "" {
		fmt.Printf(" >> Writing Publisher: %s\n", volStruct.Publisher)
		writeTagB(f, spec.Publisher, []byte(volStruct.Publisher))
	}
	if volStruct.Genre != "" {
		fmt.Printf(" >> Writing Genre: %s\n", volStruct.Genre)
		writeTagB(f, spec.Genre, []byte(volStruct.Genre))
	}
	if volStruct.CopyRight != "" {
		fmt.Printf(" >> Writing CopyRight: %s\n", volStruct.CopyRight)
		writeTagB(f, spec.Copyright, []byte(volStruct.CopyRight))
	}
	if volStruct.VolumeInfo != "" {
		fmt.Printf(" >> Writing VolumeInfo: %s\n", volStruct.VolumeInfo)
		writeTagB(f, spec.VolumeInfo, []byte(volStruct.VolumeInfo))
	}
	if volStruct.ReleaseDate != "" {
		fmt.Printf(" >> Writing Release Date: %s\n", volStruct.ReleaseDate)
		writeTagB(f, spec.ReleaseDate, []byte(volStruct.ReleaseDate))
	}
	if volStruct.CreatedDate != "" {
		fmt.Printf(" >> Writing Created Date: %s\n", volStruct.CreatedDate)
		writeTagB(f, spec.CreatedDate, []byte(volStruct.CreatedDate))
	}

	// 2. AUDIO BLOCK
	f.Write([]byte(spec.AudioData))
	adatSizePos, _ := f.Seek(0, io.SeekCurrent)
	binary.Write(f, binary.BigEndian, uint32(0))

	startAudioArea, _ := f.Seek(0, io.SeekCurrent)
	var finalTracks []TrackEntry

	for i, track := range volStruct.Content {
		// Print status manual menggantikan NewProgress
		fmt.Printf(" [%d/%d] Processing: %s\n", i+1, len(volStruct.Content), track.Title)

		inputPath := track.OriginFile
		if !filepath.IsAbs(inputPath) {
			inputPath = filepath.Join(filepath.Dir(jsonPath), inputPath)
		}

		resChan := make(chan audioengine.EncoderResult, 100)
		var trackSize uint64

		currentPos, _ := f.Seek(0, io.SeekCurrent)
		track.Offset = uint64(currentPos)

		var writeWg sync.WaitGroup
		writeWg.Add(1)
		go func() {
			defer writeWg.Done()
			for res := range resChan {
				if res.Error == nil {
					enc, _ := security.Encrypt(res.Frame, audioKey)
					binary.Write(f, binary.BigEndian, uint16(len(enc)))
					f.Write(enc)
					trackSize += uint64(2 + len(enc))
				}
			}
		}()

		dur, _ := audioengine.StreamEncodeWavToOpus(inputPath, resChan)
		close(resChan)
		writeWg.Wait()

		track.Duration = dur
		track.Size = trackSize
		track.Fingerprint = "STREAM_V2"

		finalTracks = append(finalTracks, track)

		runtime.GC()
		debug.FreeOSMemory()
	}

	// 3. UPDATE ADAT SIZE
	endAudioArea, _ := f.Seek(0, io.SeekCurrent)
	totalAdatSize := uint32(endAudioArea - startAudioArea)
	f.Seek(adatSizePos, io.SeekStart)
	binary.Write(f, binary.BigEndian, totalAdatSize)
	f.Seek(endAudioArea, io.SeekStart)

	//artwork
	InjectArtwork(f, volStruct.ArtworkPath)
	// 4. JSFD SEALING
	volStruct.Content = finalTracks
	tocBytes, _ := json.Marshal(volStruct)

	f.Write([]byte("JSFD"))
	binary.Write(f, binary.BigEndian, uint32(len(tocBytes)))
	f.Write(tocBytes)
	fmt.Println(" >> Metadata JSFD Sealed Successfully.")

	f.Sync()
	f.Close()

	security.CreateKeyLocker(finalPath, password)
	fmt.Printf("\n[SUCCESS] Volume Forged: %s\n", finalPath)
}

func writeTagB(w io.Writer, tag string, data []byte) {
	w.Write([]byte(tag))
	binary.Write(w, binary.BigEndian, uint32(len(data)))
	w.Write(data)
}

func runVolmakerInterview() (string, string, string, int) {
	rl, _ := readline.NewEx(&readline.Config{Prompt: ">> "})
	defer rl.Close()

	fmt.Printf("\n%s version %d.%d\n", app_name, version_major, version_minor)
	fmt.Printf("%s\n", developer_title)
	fmt.Printf("%s\n", developer_subtitle)
	j := ask(rl, "1. JSON Struct Path", "your-struct.json")
	d := ask(rl, "2. Destination Folder (must exist)", ".")
	p := ask(rl, "3. Password Master", "hardix2025")
	w, _ := strconv.Atoi(ask(rl, "4. Worker Threads", "2"))

	return j, d, p, w
}

func ask(rl *readline.Instance, prompt, def string) string {
	rl.SetPrompt(fmt.Sprintf("%s [%s]: ", prompt, def))
	line, _ := rl.Readline()
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}
func InjectArtwork(f *os.File, artworkPath string) {
	if artworkPath == "" {
		return
	}
	artBytes, err := os.ReadFile(artworkPath)
	if err != nil {
		fmt.Printf("\n [!] Gagal baca artwork: %v", err)
		return
	}
	fmt.Printf("\n >> Injecting Artwork: %s (%d bytes)", filepath.Base(artworkPath), len(artBytes))
	writeTagB(f, "ARTW", artBytes)
}
