package audioengine

// ApplyQuickGain melakukan normalisasi volume secara streaming tanpa buffer besar
func ApplyQuickGain(samples []int16, factor float64) {
	for i := range samples {
		val := float64(samples[i]) * factor
		if val > 32767 {
			val = 32767
		} else if val < -32768 {
			val = -32768
		}
		samples[i] = int16(val)
	}
}
