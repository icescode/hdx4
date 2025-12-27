/*
 * Copyright (c) 2025 Hardiyanto Y -Ebiet.
 * This software is part of the HDX (Hardix Audio) project.
 * This code is provided "as is", without warranty of any kind.
 */

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const (
	version_major      = 1
	version_minor      = 0
	developer_title    = "Developer Hardiyanto"
	developer_subtitle = "Build 27/12/2025 -Ebiet Version"
	usage_text         = "Usage: hdx-flac2wav -sourcepath (FLAC[s] Path) -destpath (WAV[s] Path)  [-workers 4]"
	app_name           = "HDX-Flac2wav"
)

func main() {
	sourcePath := flag.String("sourcepath", "", "Direktori sumber file FLAC")
	destPath := flag.String("destpath", "", "Direktori tujuan file WAV")
	workers := flag.Int("workers", 2, "Jumlah proses simultan (default 2)")

	flag.Parse()

	if *sourcePath == "" || *destPath == "" {
		fmt.Printf("%s version %d.%d\n", app_name, version_major, version_minor)
		fmt.Printf("%s - %s\n", developer_title, developer_subtitle)
		fmt.Printf("%s\n", usage_text)
		return
	}

	// 1. Buat direktori tujuan jika belum ada
	os.MkdirAll(*destPath, os.ModePerm)

	// 2. Kumpulkan semua file FLAC
	var files []string
	filepath.Walk(*sourcePath, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && strings.ToLower(filepath.Ext(path)) == ".flac" {
			files = append(files, path)
		}
		return nil
	})

	fmt.Printf("[Batch] Ditemukan %d file FLAC. Memulai konversi dengan %d workers...\n", len(files), *workers)

	// 3. Setup Worker Pool
	jobs := make(chan string, len(files))
	var wg sync.WaitGroup

	for w := 1; w <= *workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for path := range jobs {
				convert(path, *destPath)
			}
		}(w)
	}

	// 4. Kirim tugas ke channel
	for _, path := range files {
		jobs <- path
	}
	close(jobs)

	wg.Wait()
	fmt.Println("\n[Success] Semua konversi selesai.")
}

func convert(srcFile, destDir string) {
	fileName := filepath.Base(srcFile)
	destFile := filepath.Join(destDir, strings.TrimSuffix(fileName, filepath.Ext(fileName))+".wav")

	fmt.Printf("[Process] Konversi : %s\n", fileName)

	// Argumen FFmpeg untuk hasil optimal 48kHz Stereo sesuai spek HDX1
	// Menggunakan 'nice' di Linux/Mac untuk merendahkan prioritas CPU
	var cmd *exec.Cmd
	if runtime.GOOS != "windows" {
		cmd = exec.Command("nice", "-n", "15", "ffmpeg", "-i", srcFile, "-ar", "48000", "-ac", "2", "-y", destFile)
	} else {
		cmd = exec.Command("ffmpeg", "-i", srcFile, "-ar", "48000", "-ac", "2", "-y", destFile)
	}

	err := cmd.Run()
	if err != nil {
		fmt.Printf("[Error] Gagal konversi %s: %v\n", fileName, err)
	}
}
