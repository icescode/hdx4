package codec

import (
	"math"
)

// GenerateWaveformData membuat byte array berisi amplitudo (0-255)
// untuk memudahkan mikrokontroler menggerakkan LED/sensor.
func GenerateWaveformData(pcm []int16) []byte {
	const targetPoints = 1000 // Resolusi visual (1000 titik sepanjang lagu)
	step := len(pcm) / targetPoints
	if step == 0 {
		step = 1
	}

	waveform := make([]byte, 0, targetPoints)

	for i := 0; i < len(pcm); i += step {
		var sum float64
		count := 0
		// Hitung rata-rata energi dalam satu blok (RMS)
		for j := 0; j < step && (i+j) < len(pcm); j++ {
			val := float64(pcm[i+j])
			sum += val * val
			count++
		}

		rms := math.Sqrt(sum / float64(count))
		// Normalisasi ke skala 0-255 (1 byte)
		normalized := uint8(math.Min((rms/32768.0)*255.0*5.0, 255.0))
		waveform = append(waveform, normalized)
	}
	return waveform
}
