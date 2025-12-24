package audioengine

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// GenerateSimpleFingerprint membuat hash dari ringkasan data audio
func GenerateSimpleFingerprint(sampleSummary []int16) string {
	h := sha256.New()
	for _, s := range sampleSummary {
		binary.Write(h, binary.LittleEndian, s)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// CollectWaveformPoints mengambil sampel puncak setiap N blok untuk UI
func CollectWaveformPoints(chunk []int16, points []int16) []int16 {
	if len(chunk) == 0 {
		return points
	}
	var max int16
	for _, v := range chunk {
		if v > max {
			max = v
		}
	}
	return append(points, max)
}
