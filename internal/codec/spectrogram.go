package codec

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"

	"github.com/mjibson/go-dsp/fft"
)

// GenerateSpectrogram menciptakan biner gambar PNG dari data PCM
func GenerateSpectrogram(pcm []int16) ([]byte, error) {
	const width = 800
	const height = 200
	const fftSize = 1024

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Hitung berapa banyak sampel per kolom pixel
	step := len(pcm) / width
	if step < fftSize {
		step = fftSize
	}

	for x := 0; x < width; x++ {
		start := x * step
		if start+fftSize > len(pcm) {
			break
		}

		// Ambil potongan PCM dan konversi ke float64 untuk FFT
		window := make([]float64, fftSize)
		for i := 0; i < fftSize; i++ {
			window[i] = float64(pcm[start+i])
		}

		// Jalankan FFT
		coeffs := fft.FFTReal(window)

		// Gambar intensitas frekuensi ke pixel (Y axis)
		for y := 0; y < height; y++ {
			// Mapping index frekuensi (logarithmic atau linear)
			idx := (height - 1 - y) * (fftSize / 2) / height
			mag := math.Sqrt(real(coeffs[idx])*real(coeffs[idx]) + imag(coeffs[idx])*imag(coeffs[idx]))

			// Normalisasi intensitas ke warna (0-255)
			intensity := uint8(math.Min(mag/500, 255))
			img.Set(x, y, color.RGBA{R: intensity / 2, G: intensity, B: intensity / 2, A: 255})
		}
	}

	// Encode image ke PNG buffer
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
