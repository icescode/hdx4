package codec

import (
	"bytes"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"io"
)

// ProcessArtwork memotong gambar menjadi square dari tengah
func ProcessArtwork(r io.Reader) ([]byte, error) {
	src, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}

	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Ambil dimensi terkecil untuk membuat kotak
	size := w
	if h < w {
		size = h
	}

	// Hitung titik awal agar crop tepat di tengah
	x0 := (w - size) / 2
	y0 := (h - size) / 2

	squareImg := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(squareImg, squareImg.Bounds(), src, image.Point{x0, y0}, draw.Src)

	// Target 600x600 (di antara range 256-1024)
	targetSize := 600
	if size < targetSize {
		targetSize = size
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetSize, targetSize))

	// Resizing sederhana
	for x := 0; x < targetSize; x++ {
		for y := 0; y < targetSize; y++ {
			srcX := x * size / targetSize
			srcY := y * size / targetSize
			dst.Set(x, y, squareImg.At(srcX, srcY))
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
