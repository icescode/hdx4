package audioengine

import (
	"io"
	"os"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/hraban/opus"
)

type EncoderResult struct {
	Frame []byte
	Error error
}

func StreamEncodeWavToOpus(inputPath string, resultChan chan<- EncoderResult) (float64, error) {
	file, err := os.Open(inputPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	dec := wav.NewDecoder(file)

	// Inisialisasi Encoder Opus (Hardix Standard: 48kHz Stereo)
	enc, err := opus.NewEncoder(48000, 2, opus.AppAudio)
	if err != nil {
		return 0, err
	}

	frameSize := 960 // 20ms @ 48kHz
	channels := 2
	pcmBuf := make([]int16, frameSize*channels)
	opusBuf := make([]byte, 1500)

	// Sediakan objek IntBuffer untuk menampung data hasil baca
	// Kita baca 1 detik (96000 samples) per siklus I/O agar efisien
	intBuf := &audio.IntBuffer{
		Data:   make([]int, 48000*channels),
		Format: &audio.Format{NumChannels: channels, SampleRate: 48000},
	}

	totalSamples := 0
	for {
		// PCMBuffer sekarang menerima pointer intBuf dan mengembalikan jumlah sampel (int)
		n, err := dec.PCMBuffer(intBuf)
		if err != nil && err != io.EOF {
			return 0, err
		}

		if n == 0 {
			break
		}

		// Memecah data di intBuf.Data (sepanjang n) menjadi frame-frame Opus 20ms
		for i := 0; i < n; i += len(pcmBuf) {
			end := i + len(pcmBuf)
			actualBatchSize := len(pcmBuf)

			if end > n {
				actualBatchSize = n - i
				// Reset pcmBuf untuk padding silence
				for j := 0; j < len(pcmBuf); j++ {
					pcmBuf[j] = 0
				}
			}

			// Konversi data int ke int16
			for j := 0; j < actualBatchSize; j++ {
				pcmBuf[j] = int16(intBuf.Data[i+j])
			}

			// Encode ke Opus
			outputSize, err := enc.Encode(pcmBuf, opusBuf)
			if err != nil {
				resultChan <- EncoderResult{Error: err}
				return 0, err
			}

			frameCopy := make([]byte, outputSize)
			copy(frameCopy, opusBuf[:outputSize])
			resultChan <- EncoderResult{Frame: frameCopy}
			totalSamples += actualBatchSize
		}

		if err == io.EOF {
			break
		}
	}

	duration := float64(totalSamples) / 48000.0 / float64(channels)
	return duration, nil
}
