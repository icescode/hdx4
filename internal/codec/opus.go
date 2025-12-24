/*
	package codec

import (

	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-audio/wav"
	"github.com/hraban/opus"

)

// EncodeAudioToOpus melakukan pipeline lengkap dari WAV ke Opus Frames + Metadata Visual

	func EncodeAudioToOpus(inputPath string) ([][]byte, float64, string, []byte, []byte, []byte, error) {
		file, err := os.Open(inputPath)
		if err != nil {
			return nil, 0, "", nil, nil, nil, err
		}
		defer file.Close()

		if strings.ToLower(filepath.Ext(inputPath)) != ".wav" {
			return nil, 0, "", nil, nil, nil, fmt.Errorf("hanya mendukung format .wav 48kHz")
		}

		dec := wav.NewDecoder(file)
		buf, err := dec.FullPCMBuffer()
		if err != nil {
			return nil, 0, "", nil, nil, nil, err
		}

		// Konversi manual audio.IntBuffer ke []int16
		pcmData := make([]int16, len(buf.Data))
		for i, v := range buf.Data {
			pcmData[i] = int16(v)
		}

		// 1. Normalisasi
		pcmData = NormalizePCM(pcmData)

		// 2. Encode ke Opus
		frames, err := EncodeRawToOpus(pcmData, 48000)
		if err != nil {
			return nil, 0, "", nil, nil, nil, err
		}

		duration := float64(len(pcmData)) / 48000.0 / 2.0 // Stereo

		// 3. Metadata & Visuals
		fgv2, loh := GenerateFingerprintV2(pcmData)
		specImg, _ := GenerateSpectrogram(pcmData)
		waveData := GenerateWaveformData(pcmData)

		return frames, duration, fgv2, loh, specImg, waveData, nil
	}

// DecodeOpusToPcm merubah payload frames kembali menjadi PCM []int16

	func DecodeOpusToPcm(frames [][]byte, rate int) ([]int16, error) {
		dec, err := opus.NewDecoder(rate, 2)
		if err != nil {
			return nil, err
		}

		var fullPcm []int16
		frameSize := rate / 50 // 20ms

		for _, frame := range frames {
			out := make([]int16, frameSize*2) // Stereo
			n, err := dec.Decode(frame, out)
			if err != nil {
				return nil, err
			}
			fullPcm = append(fullPcm, out[:n*2]...)
		}

		return fullPcm, nil
	}

// EncodeRawToOpus membagi PCM menjadi frames Opus

	func EncodeRawToOpus(pcm []int16, rate int) ([][]byte, error) {
		enc, err := opus.NewEncoder(rate, 2, opus.AppAudio)
		if err != nil {
			return nil, err
		}

		frameSize := rate / 50
		sampleSize := frameSize * 2
		var frames [][]byte

		for i := 0; i < len(pcm); i += sampleSize {
			end := i + sampleSize
			var chunk []int16
			if end > len(pcm) {
				chunk = make([]int16, sampleSize)
				copy(chunk, pcm[i:])
			} else {
				chunk = pcm[i:end]
			}

			data := make([]byte, 1000)
			n, err := enc.Encode(chunk, data)
			if err != nil {
				return nil, err
			}
			frames = append(frames, data[:n])
		}
		return frames, nil
	}

// NormalizePCM melakukan Peak Normalization

	func NormalizePCM(samples []int16) []int16 {
		var max int16 = 0
		for _, s := range samples {
			absS := s
			if s < 0 {
				absS = -s
			}
			if absS > max {
				max = absS
			}
		}
		if max == 0 {
			return samples
		}

		ratio := 32760.0 / float64(max)
		for i := range samples {
			samples[i] = int16(float64(samples[i]) * ratio)
		}
		return samples
	}
*/
package codec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-audio/wav"
	"github.com/hraban/opus"
)

func EncodeAudioToOpus(inputPath string) ([][]byte, float64, string, []byte, []byte, []byte, error) {
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, 0, "", nil, nil, nil, err
	}
	defer file.Close()

	if strings.ToLower(filepath.Ext(inputPath)) != ".wav" {
		return nil, 0, "", nil, nil, nil, fmt.Errorf("hanya mendukung format .wav 48kHz")
	}

	dec := wav.NewDecoder(file)
	// OPTIMASI 1: Gunakan buffer PCM yang lebih kecil untuk konversi manual,
	// atau jika terpaksa FullPCM, kita harus segera membuang 'dec' setelahnya.
	buf, err := dec.FullPCMBuffer()
	if err != nil {
		return nil, 0, "", nil, nil, nil, err
	}

	// Konversi langsung ke int16
	pcmData := make([]int16, len(buf.Data))
	for i, v := range buf.Data {
		pcmData[i] = int16(v)
	}

	// OPTIMASI 2: Segera bebaskan memori buffer mentah dari wav decoder
	buf.Data = nil
	buf = nil

	// 1. Normalisasi
	pcmData = NormalizePCM(pcmData)

	// 2. Encode ke Opus
	frames, err := EncodeRawToOpus(pcmData, 48000)
	if err != nil {
		pcmData = nil
		return nil, 0, "", nil, nil, nil, err
	}

	duration := float64(len(pcmData)) / 48000.0 / 2.0 // Stereo

	// 3. Metadata & Visuals
	// Pastikan fungsi-fungsi ini tidak melakukan deep-copy data pcmData
	fgv2, loh := GenerateFingerprintV2(pcmData)
	specImg, _ := GenerateSpectrogram(pcmData)
	waveData := GenerateWaveformData(pcmData)

	// OPTIMASI 3: Putus referensi pcmData sebelum return agar bisa di-GC
	pcmData = nil

	return frames, duration, fgv2, loh, specImg, waveData, nil
}

func EncodeRawToOpus(pcm []int16, rate int) ([][]byte, error) {
	enc, err := opus.NewEncoder(rate, 2, opus.AppAudio)
	if err != nil {
		return nil, err
	}

	frameSize := rate / 50 // 20ms
	sampleSize := frameSize * 2
	var frames [][]byte

	// Pre-alokasi buffer untuk satu frame agar tidak alokasi di dalam loop
	tmpData := make([]byte, 1000)

	for i := 0; i < len(pcm); i += sampleSize {
		end := i + sampleSize
		var chunk []int16
		if end > len(pcm) {
			chunk = make([]int16, sampleSize)
			copy(chunk, pcm[i:])
		} else {
			chunk = pcm[i:end]
		}

		n, err := enc.Encode(chunk, tmpData)
		if err != nil {
			return nil, err
		}

		// Copy hanya data yang dihasilkan
		frame := make([]byte, n)
		copy(frame, tmpData[:n])
		frames = append(frames, frame)
	}
	return frames, nil
}

// Fungsi lainnya (NormalizePCM, dll) tetap sama
func NormalizePCM(samples []int16) []int16 {
	var max int16 = 0
	for _, s := range samples {
		absS := s
		if s < 0 {
			absS = -s
		}
		if absS > max {
			max = absS
		}
	}
	if max == 0 {
		return samples
	}

	ratio := 32760.0 / float64(max)
	for i := range samples {
		samples[i] = int16(float64(samples[i]) * ratio)
	}
	return samples
}
