package main

import (
	"fmt"
	"strings"
	"sync"
)

type Progress struct {
	total   int
	current int
	mu      sync.Mutex
}

func NewProgress(total int) *Progress {
	return &Progress{total: total}
}

func (p *Progress) Add(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current += n
	p.draw()
}

func (p *Progress) draw() {
	width := 30
	percent := float64(p.current) / float64(p.total)
	filled := int(float64(width) * percent)

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	// Menggunakan \r untuk mengembalikan kursor ke awal baris
	fmt.Printf("\r [FORGING] [%s] %d%% (%d/%d tracks)", bar, int(percent*100), p.current, p.total)

	if p.current == p.total {
		fmt.Println() // New line saat selesai
	}
}
