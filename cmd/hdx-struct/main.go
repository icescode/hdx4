package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chzyer/readline"
)

// --- Structs ---

type Session struct {
	Album       string `json:"album"`
	Artist      string `json:"artist"`
	Publisher   string `json:"publisher"`
	Copyright   string `json:"copyright"`
	Genre       string `json:"genre"`
	Date        string `json:"date"`
	WorkerCount int    `json:"worker_count"`
}

type VolumeStructure struct {
	VolumeInfo  string       `json:"volume_info"`
	CreatedDate string       `json:"created_date"`
	Album       string       `json:"album"`
	Artist      string       `json:"artist"`
	ReleaseDate string       `json:"release_date"`
	Genre       string       `json:"genre"`
	Publisher   string       `json:"publisher"`
	Copyright   string       `json:"copyright"`
	ArtworkPath string       `json:"artwork_path"`
	Content     []TrackEntry `json:"content"`
}

type TrackEntry struct {
	TrackNumber int       `json:"track_number"`
	Title       string    `json:"title"`
	Artist      string    `json:"artist"`
	ISRC        string    `json:"isrc"`
	Wave        []int     `json:"wave"`
	RMS         []float64 `json:"rms"`
	Fingerprint string    `json:"fingerprint"`
	OriginFile  string    `json:"origin_file"`
}

type Job struct {
	Path  string
	Index int
}

func main() {
	// 1. Parsing Flags (Automation Support)
	srcPtr := flag.String("sourcepath", "", "Source WAV Path")
	jsonPtr := flag.String("json", "", "JSON Output Path")
	flag.Parse()

	var config VolumeStructure
	var workerLimit int
	var finalSrcPath, finalJsonPath string

	// 2. Mode Selection
	if *srcPtr == "" {
		config, workerLimit, finalSrcPath, finalJsonPath = runInterview()
	} else {
		// Default simple mapping for CLI flags (minimalist)
		config = VolumeStructure{Album: "Unknown", Artist: "Unknown", VolumeInfo: "Output.hdxv"}
		workerLimit = 1
		finalSrcPath = *srcPtr
		finalJsonPath = *jsonPtr
	}

	// 3. Scan Files
	fmt.Printf("\n[1/3] Scanning WAV files in %s...\n", finalSrcPath)
	wavFiles := findWavs(finalSrcPath)
	if len(wavFiles) == 0 {
		fmt.Println("[ERROR] No WAV files found!")
		return
	}
	sort.Strings(wavFiles)

	// 4. Processing Workers
	jobs := make(chan Job, len(wavFiles))
	results := make(chan TrackEntry, len(wavFiles))
	var wg sync.WaitGroup

	for w := 0; w < workerLimit; w++ {
		wg.Add(1)
		go worker(&wg, jobs, results, config.Artist)
	}

	for i, path := range wavFiles {
		jobs <- Job{Path: path, Index: i + 1}
	}
	close(jobs)
	wg.Wait()
	close(results)

	// Collect and Sort
	for res := range results {
		config.Content = append(config.Content, res)
	}
	sort.Slice(config.Content, func(i, j int) bool {
		return config.Content[i].TrackNumber < config.Content[j].TrackNumber
	})

	config.CreatedDate = time.Now().Format("02:01:2006-15:04:05")

	// 5. Finalize
	outputJSON, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(finalJsonPath, outputJSON, 0644)
	fmt.Printf("\n[3/3] Success! Processed %d tracks.\nJSON: %s\n", len(wavFiles), finalJsonPath)
}

// --- High Speed Core ---

func worker(wg *sync.WaitGroup, jobs <-chan Job, results chan<- TrackEntry, defaultArtist string) {
	defer wg.Done()
	for j := range jobs {
		wave, rms, fg, err := fastProcessWav(j.Path)
		if err != nil {
			fmt.Printf(" [SKIP] %s: %v\n", j.Path, err)
			continue
		}
		results <- TrackEntry{
			TrackNumber: j.Index,
			Title:       strings.TrimSuffix(filepath.Base(j.Path), ".wav"),
			Artist:      defaultArtist,
			ISRC:        fmt.Sprintf("ID-HDX-%s", strings.ToUpper(fg[:10])),
			Wave:        wave, RMS: rms, Fingerprint: fg, OriginFile: j.Path,
		}
		fmt.Printf(" [+] Done: %s\n", filepath.Base(j.Path))
	}
}

// fastProcessWav: Direct Binary Access (No full decoding)
func fastProcessWav(path string) ([]int, []float64, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, "", err
	}
	defer f.Close()

	stat, _ := f.Stat()
	size := stat.Size()

	// Fingerprint: Head, Mid, Tail
	h := sha256.New()
	buf := make([]byte, 4096)
	f.ReadAt(buf, 0)
	h.Write(buf)
	if size > 10000 {
		f.ReadAt(buf, size/2)
		h.Write(buf)
		f.ReadAt(buf, size-4096)
		h.Write(buf)
	}
	fg := hex.EncodeToString(h.Sum(nil))

	// Audio Sampling (WAV Header is 44 bytes)
	const points = 200
	dataStart := int64(44)
	if size <= dataStart {
		return nil, nil, fg, fmt.Errorf("file too small")
	}

	step := (size - dataStart) / points
	var wave []int
	var rms []float64
	sample := make([]byte, 2)

	for i := int64(0); i < points; i++ {
		f.ReadAt(sample, dataStart+(i*step))
		// Convert to 16-bit PCM (Little Endian)
		val := int16(sample[0]) | int16(sample[1])<<8
		absVal := math.Abs(float64(val))

		wave = append(wave, int(absVal/128))
		rms = append(rms, absVal/32768.0)
	}

	return wave, rms, fg, nil
}

// --- Interview Engine ---

func runInterview() (VolumeStructure, int, string, string) {
	sess := loadSession()

	rl, _ := readline.NewEx(&readline.Config{
		Prompt: ">> ",
		AutoComplete: readline.NewPrefixCompleter(
			readline.PcItemDynamic(func(line string) []string {
				return listFiles(line)
			}),
		),
	})
	defer rl.Close()

	for {
		fmt.Println("\n=== HDX-STRUCT: INTERACTIVE MODE ===")
		album := ask(rl, "1. Album Name", sess.Album)
		artist := ask(rl, "2. Artist Name", sess.Artist)
		pub := ask(rl, "3. Publisher", sess.Publisher)
		copyr := ask(rl, "4. Copyright", sess.Copyright)
		genre := ask(rl, "5. Genre", sess.Genre)
		date := ask(rl, "6. Release Date", sess.Date)

		fmt.Println("(Tip: Use TAB to autocomplete paths)")
		artPath := askValidFile(rl, "7. Artwork Path", "")
		srcPath := askValidDir(rl, "8. Source WAV Path", "")
		jsonPath := ask(rl, "9. JSON Output Path", filepath.Join(srcPath, "struct.json"))
		volName := ask(rl, "10. Target Volume Name", strings.ReplaceAll(album, " ", "_"))

		maxCPU := runtime.NumCPU()
		wStr := ask(rl, "11. Worker Count (1-"+strconv.Itoa(maxCPU)+")", strconv.Itoa(sess.WorkerCount))
		wInt, _ := strconv.Atoi(wStr)

		// FULL REVIEW
		fmt.Println("\n--- REVIEW SELECTIONS ---")
		fmt.Printf(" [Album]     : %s\n [Artist]    : %s\n [Publisher] : %s\n [Copyright] : %s\n", album, artist, pub, copyr)
		fmt.Printf(" [Genre]     : %s\n [Date]      : %s\n [Artwork]   : %s\n", genre, date, artPath)
		fmt.Printf(" [Source]    : %s\n [JSON Dest] : %s\n [Volume]    : %s.hdxv\n [Workers]   : %d\n", srcPath, jsonPath, volName, wInt)
		fmt.Println("--------------------------")

		ans := ask(rl, "Proceed? (y) Yes / (r) Restart / (q) Quit", "y")
		if ans == "y" {
			saveSession(Session{album, artist, pub, copyr, genre, date, wInt})
			return VolumeStructure{
				Album: album, Artist: artist, Publisher: pub, Copyright: copyr,
				Genre: genre, ReleaseDate: date, ArtworkPath: artPath, VolumeInfo: volName + ".hdxv",
			}, wInt, srcPath, jsonPath
		} else if ans == "q" {
			os.Exit(0)
		}
	}
}

// Helpers
func ask(rl *readline.Instance, label, defaultVal string) string {
	rl.SetPrompt(fmt.Sprintf("%s [%s]: ", label, defaultVal))
	line, _ := rl.Readline()
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

func askValidFile(rl *readline.Instance, label, defaultVal string) string {
	for {
		p := ask(rl, label, defaultVal)
		if s, err := os.Stat(p); err == nil && !s.IsDir() {
			return p
		}
		fmt.Println(" [!] Path is not a valid file.")
	}
}

func askValidDir(rl *readline.Instance, label, defaultVal string) string {
	for {
		p := ask(rl, label, defaultVal)
		if s, err := os.Stat(p); err == nil && s.IsDir() {
			return p
		}
		fmt.Println(" [!] Path is not a valid directory.")
	}
}

func listFiles(line string) []string {
	dir := filepath.Dir(line)
	if line == "" {
		dir = "."
	}
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		name := filepath.Join(dir, e.Name())
		if strings.HasPrefix(name, line) {
			names = append(names, name)
		}
	}
	return names
}

func findWavs(root string) []string {
	var f []string
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(p), ".wav") {
			f = append(f, p)
		}
		return nil
	})
	return f
}

func loadSession() Session {
	var s Session
	home, _ := os.UserHomeDir()
	b, err := os.ReadFile(filepath.Join(home, ".hdx_struct_session"))
	if err == nil {
		json.Unmarshal(b, &s)
	}
	if s.WorkerCount == 0 {
		s.WorkerCount = 1
	}
	return s
}

func saveSession(s Session) {
	home, _ := os.UserHomeDir()
	b, _ := json.Marshal(s)
	os.WriteFile(filepath.Join(home, ".hdx_struct_session"), b, 0644)
}
