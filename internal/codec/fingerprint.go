package codec

import (
	"crypto/sha256"
	"fmt"
	"math"
)

type landmark struct {
	time int
	freq int
}

// GenerateFingerprintV2 menghasilkan FGV2 (string) dan LOH (binary)
func GenerateFingerprintV2(pcm []int16) (string, []byte) {
	const fftSize = 1024
	const stride = 512
	const segmentSize = 48000 * 2 * 5 // 5 detik audio stereo 48kHz

	var allLandmarks []landmark
	var loh []byte

	// 1. Proses per segmen 5 detik untuk List of Hash (LOH)
	for s := 0; s < len(pcm); s += segmentSize {
		end := s + segmentSize
		if end > len(pcm) {
			end = len(pcm)
		}

		segmentPCM := pcm[s:end]
		hSeg := sha256.New()

		// Deteksi Landmark di dalam segmen
		for i := 0; i+fftSize < len(segmentPCM); i += stride {
			maxMag := 0.0
			peakFreq := 0
			for j := 0; j < fftSize; j++ {
				mag := math.Abs(float64(segmentPCM[i+j]))
				if mag > maxMag {
					maxMag = mag
					peakFreq = j
				}
			}

			if maxMag > 500 {
				lm := landmark{time: i / stride, freq: peakFreq}
				allLandmarks = append(allLandmarks, lm)
				// Masukkan data landmark ke hash segmen
				hSeg.Write([]byte(fmt.Sprintf("%d-%d", lm.time, lm.freq)))
			}
		}
		// Ambil 4 byte pertama dari hash segmen untuk LOH
		loh = append(loh, hSeg.Sum(nil)[:4]...)
	}

	// 2. Generate Global Hash (FGV2) dari semua landmarks yang terkumpul
	hGlobal := sha256.New()
	for _, l := range allLandmarks {
		hGlobal.Write([]byte(fmt.Sprintf("%d|%d", l.time, l.freq)))
	}

	return fmt.Sprintf("HRDX-V2-%x", hGlobal.Sum(nil)[:12]), loh
}
