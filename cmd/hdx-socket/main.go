package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"hardix/internal/security"
)

// --- STRUCT DEFINITIONS ---
type VolumeListEntry struct {
	Line         int    `json:"line"`
	PathFileName string `json:"pathfilename"`
	Title        string `json:"title"`
}

type TrackEntry struct {
	TrackNumber int     `json:"track_number"`
	Title       string  `json:"title"`
	Offset      uint64  `json:"offset"`
	Size        uint64  `json:"size"`
	Duration    float64 `json:"duration"`
}

type VolumeStructure struct {
	Album       string       `json:"album"`
	Artist      string       `json:"artist"`
	Publisher   string       `json:"publisher"`
	Content     []TrackEntry `json:"content"`
	ReleaseDate string       `json:"release_date"`
	Genre       string       `json:"genre"`
	CopyRight   string       `json:"copyright"`
	VolumeInfo  string       `json:"volume_info"`
	CreatedDate string       `json:"created_date"`
}

type HdxSocket struct {
	configPath string
}

func formatTitleFromPath(path string) string {
	// Ambil nama file saja tanpa path dan tanpa extensi
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))

	// Ganti underscore dengan spasi
	replaced := strings.ReplaceAll(name, "_", " ")

	// Ubah ke Title Case (Camel Case dengan spasi)
	// Contoh: vocal_jazz_2012 -> Vocal Jazz 2012
	return strings.Title(strings.ToLower(replaced))
}

// --- LOGIKA DISIPLIN LIST-VOLUME ---
func (h *HdxSocket) handleListVolume(conn net.Conn) {
	data, err := os.ReadFile(h.configPath)
	if err != nil || len(data) == 0 {
		conn.Write([]byte("ERR: CONFIG_EMPTY_OR_NOT_FOUND\n"))
		return
	}

	// Multi-OS Line Splitter (Universal)
	// Kita ganti \r\n (Windows) jadi \n dulu baru kita split
	rawContent := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(rawContent, "\n")

	var result []VolumeListEntry
	lineIdx := 0

	for _, path := range lines {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}

		// DISIPLIN: Cek existing file dan key nya (Pairing)
		datPath := strings.TrimSuffix(path, ".hdxv") + "_keys.dat"
		_, errHdxv := os.Stat(path)
		_, errDat := os.Stat(datPath)

		// Hanya masukkan ke list jika file hdxv DAN .dat nya ada
		if errHdxv == nil && errDat == nil {
			// DISIPLIN: Lakukan matchmaking singkat (Opsional jika ingin super aman)
			_, errAuth := security.UnlockKeyLocker(path, datPath, "")
			if errAuth == nil {
				entry := VolumeListEntry{
					Line:         lineIdx,
					PathFileName: path,
					Title:        formatTitleFromPath(path),
				}
				result = append(result, entry)
			}
		}
		lineIdx++
	}

	if len(result) == 0 {
		conn.Write([]byte("ERR: NO_VALID_VOLUMES_FOUND\n"))
		return
	}

	jsonRes, _ := json.Marshal(result)
	conn.Write([]byte("LIST-OK|"))
	conn.Write(jsonRes)
	conn.Write([]byte("\n"))
}

func NewHdxSocket() *HdxSocket {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".hdx-socket_list")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.Create(path)
	}
	return &HdxSocket{configPath: path}
}

// --- FUNGSI DISIPLIN: CEK DUPLIKASI ---
func (h *HdxSocket) isAlreadyInList(path string) bool {
	data, err := os.ReadFile(h.configPath)
	if err != nil {
		return false
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == strings.TrimSpace(path) {
			return true
		}
	}
	return false
}

// --- LOGIKA UTAMA ADD-VOLUME ---
func (h *HdxSocket) handleAddVolume(conn net.Conn, hdxvPath string) {
	hdxvPath = strings.TrimSpace(hdxvPath)

	// 1. DISIPLIN: Cek Duplikasi
	if h.isAlreadyInList(hdxvPath) {
		conn.Write([]byte("ERR: ALREADY_EXISTS_IN_LIST\n"))
		return
	}

	// 2. DISIPLIN: Cek Pair File
	datPath := strings.TrimSuffix(hdxvPath, ".hdxv") + "_keys.dat"
	if _, err := os.Stat(hdxvPath); err != nil {
		conn.Write([]byte("ERR: HDXV_NOT_FOUND\n"))
		return
	}
	if _, err := os.Stat(datPath); err != nil {
		conn.Write([]byte("ERR: KEY_DAT_NOT_FOUND\n"))
		return
	}

	// 3. DISIPLIN: Matchmaking Password
	albumPass, err := security.UnlockKeyLocker(hdxvPath, datPath, "")
	if err != nil {
		conn.Write([]byte(fmt.Sprintf("ERR: AUTH_FAILED|%v\n", err)))
		return
	}

	// 4. EKSTRAKSI METADATA (Tail Scanning 10MB)
	f, err := os.Open(hdxvPath)
	if err != nil {
		conn.Write([]byte("ERR: CANNOT_OPEN_FILE\n"))
		return
	}
	defer f.Close()

	stat, _ := f.Stat()
	searchLimit := int64(10 * 1024 * 1024)
	if searchLimit > stat.Size() {
		searchLimit = stat.Size()
	}
	f.Seek(stat.Size()-searchLimit, io.SeekStart)
	raw, _ := io.ReadAll(f)
	rawStr := string(raw)

	jsfdIdx := strings.LastIndex(rawStr, "JSFD")
	if jsfdIdx == -1 {
		conn.Write([]byte("ERR: JSFD_MARKER_NOT_FOUND\n"))
		return
	}

	var vol VolumeStructure
	if err := json.Unmarshal(raw[jsfdIdx+8:], &vol); err != nil {
		conn.Write([]byte("ERR: JSON_PARSING_FAILED\n"))
		return
	}

	var artSize uint32 = 0
	artIdx := strings.LastIndex(rawStr, "ARTW")
	if artIdx != -1 {
		sizeStart := artIdx + 4
		if len(raw) >= sizeStart+4 {
			artSize = binary.BigEndian.Uint32(raw[sizeStart : sizeStart+4])
		}
	}

	// 5. KIRIM PAYLOAD BALASAN
	response := map[string]interface{}{
		"album":        vol.Album,
		"publisher":    vol.Publisher,
		"genre":        vol.Genre,
		"copyright":    vol.CopyRight,
		"artwork_size": artSize,
		"artist":       vol.Artist,
		"tracks":       vol.Content,
		"auth_pass":    albumPass,
	}

	jsonRes, _ := json.Marshal(response)
	conn.Write([]byte("ADD-OK|"))
	conn.Write(jsonRes)
	conn.Write([]byte("\n"))

	// 6. DISIPLIN: Simpan ke list
	fConfig, _ := os.OpenFile(h.configPath, os.O_APPEND|os.O_WRONLY, 0644)
	defer fConfig.Close()
	fConfig.WriteString(hdxvPath + "\n")

	fmt.Printf("[SYSTEM] Added: %s (Album: %s)\n", hdxvPath, vol.Album)
}

func (h *HdxSocket) handleClient(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 0 {
			continue
		}

		cmd := strings.ToUpper(parts[0])
		switch cmd {
		case "ADD-VOLUME":
			if len(parts) > 1 {
				h.handleAddVolume(conn, parts[1])
			} else {
				conn.Write([]byte("ERR: PATH_REQUIRED\n"))
			}
		case "HEART-BEAT":
			conn.Write([]byte("ALIVE\n"))
		case "STOP":
			conn.Write([]byte("STOPPED\n"))
		case "LIST-VOLUME":
			h.handleListVolume(conn)
		case "LIST-HDXV":
			my_index, _ := strconv.Atoi(parts[1])
			h.handleListTracksByIndex(conn, my_index)

		default:
			conn.Write([]byte("ERR: UNKNOWN_COMMAND\n"))
		}
	}
}
func (h *HdxSocket) handleListTracksByIndex(conn net.Conn, index int) {
	// 1. Baca daftar volume (Multi-OS Line Splitting)
	data, err := os.ReadFile(h.configPath)
	if err != nil {
		conn.Write([]byte("ERR: CONFIG_NOT_FOUND\n"))
		return
	}
	rawContent := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(rawContent, "\n")

	// 2. Validasi Index
	if index < 0 || index >= len(lines) || strings.TrimSpace(lines[index]) == "" {
		conn.Write([]byte("ERR: INDEX_OUT_OF_RANGE\n"))
		return
	}
	hdxvPath := strings.TrimSpace(lines[index])

	// 3. Disiplin: Cek Pairing & Matchmaking [cite: 1, 3]
	datPath := strings.TrimSuffix(hdxvPath, ".hdxv") + "_keys.dat"
	if _, err := security.UnlockKeyLocker(hdxvPath, datPath, ""); err != nil {
		conn.Write([]byte("ERR: AUTH_FAILED_OR_FILE_MISSING\n"))
		return
	}

	// 4. Ekstraksi JSFD (Data Lagu)
	f, _ := os.Open(hdxvPath)
	defer f.Close()
	stat, _ := f.Stat()
	f.Seek(stat.Size()-int64(10*1024*1024), io.SeekStart)
	raw, _ := io.ReadAll(f)

	jsfdIdx := strings.LastIndex(string(raw), "JSFD")
	var vol VolumeStructure
	json.Unmarshal(raw[jsfdIdx+8:], &vol)

	// 5. Output JSON: Track Number, Title, Artist, Duration
	// Sesuai permintaan: Minimalis tapi lengkap untuk UI
	type TrackResponse struct {
		No       int     `json:"track_number"`
		Title    string  `json:"title"`
		Artist   string  `json:"artist"`
		Duration float64 `json:"duration"`
	}

	var tracks []TrackResponse
	for _, t := range vol.Content {
		tracks = append(tracks, TrackResponse{
			No:       t.TrackNumber,
			Title:    t.Title,
			Artist:   vol.Artist, // Artist diambil dari Global Volume Metadata
			Duration: t.Duration,
		})
	}

	jsonRes, _ := json.Marshal(tracks)
	conn.Write([]byte("TRACKS-OK|"))
	conn.Write(jsonRes)
	conn.Write([]byte("\n"))
}
func main() {
	server := NewHdxSocket() // Variabel 'server' didefinisikan di sini
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Printf("Fatal Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("======================================")
	fmt.Println(" HDX-SOCKET ENGINE PRO - READY")
	fmt.Println(" Listening on :8080")
	fmt.Println("======================================")

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		// FIX: Menggunakan 'server' bukan 's'
		go server.handleClient(conn)
	}
}
