package spec

var (
	// Masing-masing 64 karakter random
	r1 = "x8A2bN9mQpL5vWcE1zY7uI0oK4jH3gD6fS9dS8aA7qP6wO5eI4rU3tY2yT1xR0bV9"
	r2 = "M1nB2vC3xC4zZ5lK6jJ7hH8gG9fF0dD1sS2aA3pP4oO5iI6uU7yY8tT9rR0eE1wW2"
	r3 = "Q9qW8eE7rR6tT5yY4uU3iI2oO1pP0aA1sS2dD3fF4gG5hH6jJ7kK8lL9zZ0xX1cC2"

	// MasterBfKey gabungan rand1+rand2+rand3
	MasterBfKey = r1 + r2 + r3
)

const (
	// === IDENTITY & VERSIONING ===
	VersionV2 = "2.0.0"

	// === MAGIC NUMBERS (MODERN HDX2 - LOSSLESS) ===
	MagicV2       = "HARDIX02"
	VolumeMagicV2 = "HDXV02"
	BfKeyMagicV2  = "HRDXBF02"

	// === SECURITY & ENGINE SPECS ===
	RandomPasswordLen = 32
	NonceSize         = 12
	SampleRate        = 48000
	Channels          = 2
	FrameSize         = 20

	// === TLV TAGS (Refleksi JSON Struct) ===
	Salt         = "SALT"
	Signature    = "SIGN"
	Artwork      = "ARTW" // dari key "artwork_path"
	VolumeInfo   = "VOLI" // dari key "volume_info"
	CreatedDate  = "CRDT" // dari key "created_date"
	Album        = "ALBM" // dari key "album"
	ReleaseDate  = "RLSD" // dari key "release_date"
	Publisher    = "PUBL" // dari key "publisher"
	Copyright    = "CPRT" // dari key "copyright"
	JsonFileData = "JSFD" // isi file .json utuh
	Genre        = "GENR" // dari key "genre"

	// Tag tambahan untuk stream audio
	AudioData   = "AUDI"
	TableOfCont = "TTOC"
)
