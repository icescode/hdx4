/*
 * Copyright (c) 2025 Hardiyanto Y -Ebiet.
 * This software is part of the HDX (Hardix Audio) project.
 * This code is provided "as is", without warranty of any kind.
 */

package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	version_minor      = 0
	version_major      = 1
	developer_title    = "Developer Hardiyanto"
	developer_subtitle = "Build 27/12/2025 Ebiet Version"
	app_name           = "HDX-Meta"
	general_usage      = "Usage: ./hdx-meta -hdxv <path file name .hdxv>"
	json_dump_usage    = "Usage: ./hdx-meta -hdxv <path file name .hdxv> -jsondump"
	art_dump_usage     = "Usage: ./hdx-meta -hdxv <path file name .hdxv> -artdump"
)

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

type TrackEntry struct {
	TrackNumber int     `json:"track_number"`
	Title       string  `json:"title"`
	Offset      uint64  `json:"offset"`
	Size        uint64  `json:"size"`
	Duration    float64 `json:"duration"`
}

func main() {
	// pathFlag := flag.String("path", "", "Path file .hdxv")
	pathFlag := flag.String("hdxv", "", "full path file name of .hdxv file")
	jsonDump := flag.Bool("jsondump", false, "dump JSON JSFD content")
	artDump := flag.Bool("artdump", false, "view artwork")
	flag.Parse()

	if *pathFlag == "" {
		fmt.Printf("\n%s %d.%d\n", app_name, version_major, version_minor)
		fmt.Printf("%s %s\n", developer_title, developer_subtitle)
		fmt.Printf("%s\n", general_usage)
		fmt.Printf("%s\n", json_dump_usage)
		fmt.Printf("%s\n", art_dump_usage)
		return
	}

	f, err := os.Open(*pathFlag)
	if err != nil {
		fmt.Printf("Gagal buka file: %v\n", err)
		return
	}
	defer f.Close()

	stat, _ := f.Stat()
	// Cari marker di 10MB terakhir (dinaikkan dari 5MB untuk jaga-jaga kalau artwork besar)
	searchSize := int64(10 * 1024 * 1024)
	if searchSize > stat.Size() {
		searchSize = stat.Size()
	}

	f.Seek(stat.Size()-searchSize, io.SeekStart)
	raw, _ := io.ReadAll(f)
	rawStr := string(raw)

	// 1. Logic Cari JSFD (Asli)
	idx := strings.LastIndex(rawStr, "JSFD")
	if idx == -1 {
		fmt.Println("[!] Marker JSFD tidak ditemukan.")
		return
	}
	jsonRawData := raw[idx+8:]
	var vol VolumeStructure
	json.Unmarshal(jsonRawData, &vol)

	// 2. Logic Cari Artwork (Tambahan Baru)
	var artInfo string = "Tidak Ada Artwork"
	if *artDump {
		artIdx := strings.LastIndex(rawStr, "ARTW")
		if artIdx != -1 {
			// Baca 4 byte setelah "ARTW" untuk dapet ukurannya
			sizeStart := artIdx + 4
			if len(raw) >= sizeStart+4 {
				artSize := binary.BigEndian.Uint32(raw[sizeStart : sizeStart+4])

				// Deteksi Tipe File via Magic Numbers
				imgDataStart := sizeStart + 4
				mimeType := "unknown"
				if len(raw) >= imgDataStart+4 {
					header := raw[imgDataStart : imgDataStart+4]
					if header[0] == 0xFF && header[1] == 0xD8 {
						mimeType = "jpg/jpeg"
					} else if header[0] == 0x89 && string(header[1:4]) == "PNG" {
						mimeType = "png"
					}
				}

				artInfo = fmt.Sprintf("Artwork OK\n Tipe          : %s\n File Size     : %s", mimeType, formatSize(int64(artSize)))
			}
		}
	}

	// TAMPILAN OUTPUT
	fmt.Println(strings.Repeat("=", 75))
	fmt.Printf(" VOLUME INFO   : %s\n", vol.VolumeInfo)
	fmt.Printf(" ALBUM         : %s\n", vol.Album)
	fmt.Printf(" ARTIST        : %s\n", vol.Artist)

	if *artDump {
		fmt.Printf(" ARTWORK       : %s\n", artInfo)
	}

	fmt.Printf(" RELEASE DATE  : %s\n", vol.ReleaseDate)
	fmt.Printf(" PUBLISHER     : %s\n", vol.Publisher)
	fmt.Printf(" GENRE         : %s\n", vol.Genre)
	fmt.Printf(" COPYRIGHT     : %s\n", vol.CopyRight)
	fmt.Printf(" CREATED DATE  : %s\n", vol.CreatedDate)
	fmt.Println(strings.Repeat("-", 75))
	fmt.Printf(" %-3s | %-30s | %-10s | %-10s\n", "NO", "TRACK TITLE", "DURATION", "SIZE")
	fmt.Println(strings.Repeat("-", 75))

	for _, t := range vol.Content {
		min := int(t.Duration) / 60
		sec := int(t.Duration) % 60
		fmt.Printf(" %2d  | %-30s | %02d:%02d      | %.2f MB\n",
			t.TrackNumber, t.Title, min, sec, float64(t.Size)/(1024*1024))
	}
	fmt.Println(strings.Repeat("=", 75))

	if *jsonDump {
		fmt.Printf("\n%s versi %d.%d\n", app_name, version_major, version_minor)
		fmt.Printf("\n%s %s\n", developer_title, developer_subtitle)
		fmt.Println(string(jsonRawData))
		fmt.Println("=== [END DUMP] ===")
	}
}

// Helper untuk format size yang human friendly
func formatSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	if exp == 0 {
		return fmt.Sprintf("%.2f Kb", float64(b)/float64(unit))
	}
	return fmt.Sprintf("%.2f Mb", float64(b)/float64(div))
}
