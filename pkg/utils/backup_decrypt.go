package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

func DecryptBackupBase64FileToTemp(encFilePath, password string) (string, error) {
	if strings.TrimSpace(encFilePath) == "" {
		return "", fmt.Errorf("empty backup file path")
	}
	if strings.TrimSpace(password) == "" {
		return "", fmt.Errorf("empty backup password")
	}

	encoded, err := os.ReadFile(encFilePath)
	if err != nil {
		return "", err
	}
	payload, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		return "", fmt.Errorf("decode base64 backup failed: %w", err)
	}

	key := sha256.Sum256([]byte(password))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(payload) < gcm.NonceSize() {
		return "", fmt.Errorf("invalid encrypted payload")
	}

	nonce := payload[:gcm.NonceSize()]
	ciphertext := payload[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt backup failed: %w", err)
	}

	tempFile, err := os.CreateTemp("", "duty-log-backup-*.zip")
	if err != nil {
		return "", err
	}
	if err := tempFile.Chmod(0o600); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
		return "", err
	}
	if _, err := tempFile.Write(plaintext); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
		return "", err
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempFile.Name())
		return "", err
	}
	return tempFile.Name(), nil
}
