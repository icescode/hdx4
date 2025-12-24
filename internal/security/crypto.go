package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"

	"hardix/pkg/spec"

	"golang.org/x/crypto/pbkdf2"
)

// DeriveKey menghasilkan kunci 32-byte dari password dan salt
func DeriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, 4096, 32, sha256.New)
}

// Encrypt mengenkripsi data menggunakan AES-GCM dengan Random Nonce
func Encrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, data, nil), nil
}

// Decrypt mendekripsi data menggunakan AES-GCM
func Decrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, io.ErrUnexpectedEOF
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// CreateKeyLocker membuat file .dat (Key Locker) yang berisi password terenkripsi Master Key
func CreateKeyLocker(hdxvPath, password string) error {
	// 1. Siapkan kunci enkripsi locker menggunakan MasterBfKey dari spec
	keyForDat := DeriveKey(spec.MasterBfKey, []byte(spec.Salt))

	// 2. Enkripsi password album
	encryptedPass, err := Encrypt([]byte(password), keyForDat)
	if err != nil {
		return err
	}

	// 3. Tentukan nama file (.hdxv -> _keys.dat)
	datPath := strings.TrimSuffix(hdxvPath, ".hdxv") + "_keys.dat"

	f, err := os.Create(datPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// 4. Tulis Magic Number Locker dan data terenkripsi
	f.Write([]byte(spec.BfKeyMagicV2)) // "HRDXBF02"
	f.Write(encryptedPass)

	return nil
}
func UnlockKeyLocker(hdxvPath, lockerPath, masterPass string) (string, error) {
	// masterPass bisa digunakan jika ingin menambah layer enkripsi ekstra
	// Untuk sekarang kita pakai logika yang sudah ada
	data, err := os.ReadFile(lockerPath)
	if err != nil {
		return "", err
	}

	if len(data) < 8 || string(data[:8]) != spec.BfKeyMagicV2 {
		return "", fmt.Errorf("bukan file key locker Hardix yang valid")
	}

	keyForDat := DeriveKey(spec.MasterBfKey, []byte(spec.Salt))
	dec, err := Decrypt(data[8:], keyForDat)
	if err != nil {
		return "", err
	}

	return string(dec), nil
}
