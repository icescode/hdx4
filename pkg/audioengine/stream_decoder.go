package audioengine

import (
	"github.com/hraban/opus"
)

type StreamDecoder struct {
	dec *opus.Decoder
}

func NewStreamDecoder(rate, channels int) (*StreamDecoder, error) {
	d, err := opus.NewDecoder(rate, channels)
	if err != nil {
		return nil, err
	}
	return &StreamDecoder{dec: d}, nil
}

func (sd *StreamDecoder) DecodeFrame(frame []byte, outPcm []int16) (int, error) {
	return sd.dec.Decode(frame, outPcm)
}
