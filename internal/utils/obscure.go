package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

// obscureKey injected at build time:
// go build -ldflags="-X github.com/BrunoTulio/pgopher/internal/utils.obscureKey=your-secret"
var obscureKey = ""

func init() {
	if obscureKey == "" {
		obscureKey = "dev-mode-key"
	}
}

// deriveKey normaliza qualquer string para 32 bytes (AES-256)
func deriveKey(password string) []byte {
	hash := sha256.Sum256([]byte(password))
	return hash[:]
}

// MustObscure encrypts plaintext e retorna XXX:base64
func MustObscure(s string) string {
	key := deriveKey(obscureKey)

	block, err := aes.NewCipher(key)
	if err != nil {
		panic(fmt.Sprintf("AES cipher failed: %v", err))
	}

	plaintext := []byte(s)
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]

	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		panic(fmt.Sprintf("failed to generate IV: %v", err))
	}

	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return "XXX:" + base64.StdEncoding.EncodeToString(ciphertext)
}

// Reveal decrypta XXX:base64 e retorna plaintext
func Reveal(s string) (string, error) {
	// Remove espaços e quebras de linha
	s = strings.TrimSpace(s)

	// Se não tem prefixo XXX:, assume que é plaintext
	if !strings.HasPrefix(s, "XXX:") {
		return s, nil
	}

	key := deriveKey(obscureKey)

	// Remove prefixo XXX:
	encoded := s[4:]
	if encoded == "" {
		return "", errors.New("empty obscured string after XXX: prefix")
	}

	// Decode base64
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode failed: %w", err)
	}

	// Valida tamanho mínimo (IV + pelo menos 1 byte)
	if len(data) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}

	// Cria cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("AES cipher failed: %w", err)
	}

	// Extrai IV e ciphertext
	iv := data[:aes.BlockSize]
	ciphertext := data[aes.BlockSize:]

	// Decrypta
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return string(ciphertext), nil
}

// MustReveal decrypta ou entra em panic se falhar
func MustReveal(s string) string {
	result, err := Reveal(s)
	if err != nil {
		panic(fmt.Sprintf("failed to reveal: %v", err))
	}
	return result
}
