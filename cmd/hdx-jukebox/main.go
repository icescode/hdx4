package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"hardix/internal/security"
	"hardix/pkg/spec"

	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/hraban/opus"
)

// --- STRUCTS ---
type VolumeStructure struct {
	Album     string       `json:"album"`
	Artist    string       `json:"artist"`
	Publisher string       `json:"publisher"`
	Content   []TrackEntry `json:"content"`
	Genre     string       `json:"genre"`
	CopyRight string       `json:"copyright"`
}

type TrackEntry struct {
	TrackNumber int     `json:"track_number"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Offset      uint64  `json:"offset"`
	Duration    float64 `json:"duration"`
}

type AlbumEntry struct {
	HdxvPath string
	DatPath  string
	Meta     VolumeStructure
}

type lazyStreamer struct {
	file   *os.File
	offset int64
	key    []byte
	dec    *opus.Decoder
	buffer [][2]float64
	paused bool
}

var foundAlbums []AlbumEntry

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ./hdx-jukebox <path_to_file.hdxv>")
		return
	}

	hdxvPath := os.Args[1]
	base := strings.TrimSuffix(hdxvPath, ".hdxv")
	datPath := base + "_keys.dat"

	meta, err := readJSFDTrailer(hdxvPath)
	if err != nil {
		fmt.Printf("[!] Error metadata: %v\n", err)
		return
	}

	masterStr := strings.TrimSpace(spec.MasterBfKey)
	keyForDat := security.DeriveKey(masterStr, []byte(spec.Salt))
	datData, _ := os.ReadFile(datPath)
	decPass, _ := security.Decrypt(datData[8:], keyForDat)
	audioKey := security.DeriveKey(string(decPass), []byte(spec.Salt))

	foundAlbums = append(foundAlbums, AlbumEntry{HdxvPath: hdxvPath, DatPath: datPath, Meta: meta})

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cleanupAndExit()
	}()

	runJukebox(audioKey)
}

func runJukebox(audioKey []byte) {
	InitTerminal()
	defer CleanupTerminal()

	cursor := 0
	redraw := true
	for {
		if redraw {
			fmt.Print("\033[H\033[2J")
			fmt.Printf("HDX JUKEBOX [%d Album Loaded]\n", len(foundAlbums))
			fmt.Println("-------------------------------------------")
			for i, a := range foundAlbums {
				prefix := "  "
				if i == cursor {
					prefix = "> "
				}
				fmt.Printf("%sAlbum     : %s\n", prefix, a.Meta.Album)
				fmt.Printf("  Publisher : %s\n", a.Meta.Publisher)
			}
			fmt.Println("-------------------------------------------")
			fmt.Println(" [ENTER] Putar | [Q] Keluar")
			redraw = false
		}

		buf := make([]byte, 1)
		n, _ := os.Stdin.Read(buf)
		if n > 0 {
			char := strings.ToLower(string(buf[0]))
			if char == "q" {
				return
			}
			if buf[0] == 10 || buf[0] == 13 {
				playAlbum(foundAlbums[cursor], audioKey)
				redraw = true
			}
		}
	}
}

func playAlbum(album AlbumEntry, audioKey []byte) {
	sr := beep.SampleRate(48000)
	speaker.Init(sr, sr.N(time.Millisecond*100))

	inputChan := make(chan string)
	go func() {
		b := make([]byte, 1)
		for {
			n, _ := os.Stdin.Read(b)
			if n > 0 {
				inputChan <- strings.ToLower(string(b))
			}
		}
	}()

	for i := 0; i < len(album.Meta.Content); i++ {
		track := album.Meta.Content[i]
		f, _ := os.Open(album.HdxvPath)

		lStreamer := &lazyStreamer{file: f, offset: int64(track.Offset), key: audioKey}
		lStreamer.init()

		done := make(chan bool, 1)
		stopTicker := make(chan bool, 1)

		// Gunakan variabel lokal untuk status agar ticker tidak salah ambil data
		isPaused := false

		speaker.Play(beep.Seq(lStreamer, beep.Callback(func() { done <- true })))

		go func(tInfo TrackEntry, trackIdx int) {
			t := time.NewTicker(250 * time.Millisecond)
			defer t.Stop()
			for {
				select {
				case <-stopTicker:
					return
				case <-t.C:
					fmt.Print("\033[H\033[J")
					fmt.Println("=== HDX PLAYER ACTIVE ===")

					fmt.Printf("▶ Track %d/%d : %s [%02d:%02d]\n", trackIdx+1, len(album.Meta.Content), tInfo.Title, int(tInfo.Duration)/60, int(tInfo.Duration)%60)
					status := "PLAYING"
					speaker.Lock()
					if lStreamer.paused {
						status = "PAUSED "
					}
					speaker.Unlock()
					fmt.Printf("Status %s\n", status)
					fmt.Println("[P] Pause [N] Next [Q] Quit")
				}
			}
		}(track, i)

		nextTrack := false
		for !nextTrack {
			select {
			case <-done:
				nextTrack = true
			case key := <-inputChan:
				if key == "p" {
					speaker.Lock()
					isPaused = !isPaused
					lStreamer.paused = isPaused
					speaker.Unlock()
				} else if key == "n" {
					nextTrack = true
				} else if key == "q" {
					cleanupAndExit()
				}
			}
		}

		stopTicker <- true
		speaker.Clear()
		f.Close()
		// Jeda singkat antar lagu agar transisi halus
		time.Sleep(200 * time.Millisecond)
	}
}

// --- CORE UTILS ---

func InitTerminal() {
	exec.Command("stty", "-F", "/dev/tty", "cbreak", "min", "0", "-echo").Run()
	fmt.Print("\033[?25l")
}

func CleanupTerminal() {
	exec.Command("stty", "-F", "/dev/tty", "sane").Run()
	fmt.Print("\033[?25h")
}

func cleanupAndExit() {
	CleanupTerminal()
	fmt.Print("\033[H\033[2J")
	os.Exit(0)
}

func readJSFDTrailer(path string) (VolumeStructure, error) {
	var vol VolumeStructure
	f, _ := os.Open(path)
	if f == nil {
		return vol, fmt.Errorf("file not found")
	}
	defer f.Close()
	stat, _ := f.Stat()
	f.Seek(stat.Size()-int64(2*1024*1024), io.SeekStart)
	raw, _ := io.ReadAll(f)
	idx := strings.LastIndex(string(raw), "JSFD")
	if idx == -1 {
		return vol, fmt.Errorf("no jsfd")
	}
	json.Unmarshal(raw[idx+8:], &vol)
	return vol, nil
}

func (l *lazyStreamer) init() {
	l.dec, _ = opus.NewDecoder(48000, 2)
	l.file.Seek(l.offset, io.SeekStart)
}

func (l *lazyStreamer) Stream(samples [][2]float64) (int, bool) {
	// Proteksi status paused agar tidak looping audio
	if l.paused {
		for i := range samples {
			samples[i] = [2]float64{0, 0}
		}
		return len(samples), true
	}

	filled := 0
	for filled < len(samples) {
		if len(l.buffer) == 0 {
			var fLen uint16
			if err := binary.Read(l.file, binary.BigEndian, &fLen); err != nil {
				return filled, false
			}
			enc := make([]byte, fLen)
			io.ReadFull(l.file, enc)
			dec, _ := security.Decrypt(enc, l.key)
			out := make([]int16, 5760*2)
			n, _ := l.dec.Decode(dec, out)
			for i := 0; i < n; i++ {
				l.buffer = append(l.buffer, [2]float64{float64(out[i*2]) / 32768.0, float64(out[i*2+1]) / 32768.0})
			}
		}
		toCopy := len(l.buffer)
		if toCopy > len(samples)-filled {
			toCopy = len(samples) - filled
		}
		copy(samples[filled:], l.buffer[:toCopy])
		l.buffer = l.buffer[toCopy:]
		filled += toCopy
	}
	return filled, true
}
func (l *lazyStreamer) Err() error { return nil }
