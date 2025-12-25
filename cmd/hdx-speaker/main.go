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

	foundAlbums = append(foundAlbums, AlbumEntry{
		HdxvPath: hdxvPath,
		DatPath:  datPath,
		Meta:     meta,
	})

	// CTRL+C handler
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cleanupAndExit()
	}()

	// === AUTOPLAY ===
	playAlbum(foundAlbums[0], audioKey)

	// === AUTO EXIT ===
	cleanupAndExit()
}

func playAlbum(album AlbumEntry, audioKey []byte) {
	InitTerminal()
	defer CleanupTerminal()

	sr := beep.SampleRate(48000)
	speaker.Init(sr, sr.N(time.Millisecond*100))

	totalTracks := len(album.Meta.Content)

	for i := 0; i < totalTracks; i++ {
		track := album.Meta.Content[i]
		trackNo := i + 1 // ✅ FIX: nilai final, bukan loop variable

		f, _ := os.Open(album.HdxvPath)

		lStreamer := &lazyStreamer{
			file:   f,
			offset: int64(track.Offset),
			key:    audioKey,
		}
		lStreamer.init()

		done := make(chan struct{})
		stopTicker := make(chan struct{})

		speaker.Play(beep.Seq(
			lStreamer,
			beep.Callback(func() {
				close(done)
			}),
		))

		// === INFO TICKER (FIXED) ===
		go func(tInfo TrackEntry, trackNo int, stop <-chan struct{}) {
			t := time.NewTicker(250 * time.Millisecond)
			defer t.Stop()

			for {
				select {
				case <-stop:
					return
				case <-t.C:
					fmt.Print("\033[H\033[J")
					fmt.Println("=== HDX PLAYER AUTOPLAY ===")
					fmt.Printf(
						"▶ Track %d/%d : %s [%02d:%02d]\n",
						trackNo,
						totalTracks,
						tInfo.Title,
						int(tInfo.Duration)/60,
						int(tInfo.Duration)%60,
					)
					fmt.Println("Status PLAYING")
				}
			}
		}(track, trackNo, stopTicker)

		// tunggu track selesai
		<-done

		// stop ticker dengan BENAR
		close(stopTicker)

		speaker.Clear()
		f.Close()
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
				l.buffer = append(
					l.buffer,
					[2]float64{
						float64(out[i*2]) / 32768.0,
						float64(out[i*2+1]) / 32768.0,
					},
				)
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
