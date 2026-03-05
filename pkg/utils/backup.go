package utils

import (
	"archive/zip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"duty-log-system/internal/config"
)

type BackupResult struct {
	FilePath string
	Password string
}

func CreateDatabaseBackup(ctx context.Context, appConfig config.AppConfig, outputDir string) (BackupResult, error) {
	if strings.TrimSpace(outputDir) == "" {
		outputDir = "backups"
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return BackupResult{}, err
	}

	timestamp := time.Now().Format("20060102-150405")
	sqlFilePath := filepath.Join(outputDir, fmt.Sprintf("duty-log-%s.sql", timestamp))
	zipFilePath := filepath.Join(outputDir, fmt.Sprintf("duty-log-%s.zip", timestamp))
	encryptedFilePath := zipFilePath + ".enc"

	dumpCmd := exec.CommandContext(ctx,
		"pg_dump",
		"-h", appConfig.DBHost,
		"-p", appConfig.DBPort,
		"-U", appConfig.DBUser,
		"-d", appConfig.DBName,
		"-f", sqlFilePath,
	)
	dumpCmd.Env = append(os.Environ(), "PGPASSWORD="+appConfig.DBPassword)
	if output, err := dumpCmd.CombinedOutput(); err != nil {
		return BackupResult{}, fmt.Errorf("pg_dump failed: %w, output: %s", err, strings.TrimSpace(string(output)))
	}

	if err := zipSingleFile(sqlFilePath, zipFilePath); err != nil {
		return BackupResult{}, err
	}

	password, err := generateRandomPassword(16)
	if err != nil {
		return BackupResult{}, err
	}

	if err := encryptFileToBase64(zipFilePath, encryptedFilePath, password); err != nil {
		return BackupResult{}, err
	}

	_ = os.Remove(sqlFilePath)
	_ = os.Remove(zipFilePath)

	return BackupResult{
		FilePath: encryptedFilePath,
		Password: password,
	}, nil
}

func zipSingleFile(sourceFilePath, zipFilePath string) error {
	sourceFile, err := os.Open(sourceFilePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	zipFile, err := os.Create(zipFilePath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	entry, err := zipWriter.Create(filepath.Base(sourceFilePath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(entry, sourceFile); err != nil {
		return err
	}
	return nil
}

func encryptFileToBase64(sourceFilePath, encryptedPath, password string) error {
	raw, err := os.ReadFile(sourceFilePath)
	if err != nil {
		return err
	}

	key := sha256.Sum256([]byte(password))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	ciphertext := gcm.Seal(nil, nonce, raw, nil)
	payload := append(nonce, ciphertext...)
	encoded := base64.StdEncoding.EncodeToString(payload)

	return os.WriteFile(encryptedPath, []byte(encoded), 0o600)
}

func generateRandomPassword(length int) (string, error) {
	if length <= 0 {
		length = 16
	}
	const alphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz"

	randomBytes := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, randomBytes); err != nil {
		return "", err
	}

	var b strings.Builder
	b.Grow(length)
	for _, randomByte := range randomBytes {
		b.WriteByte(alphabet[int(randomByte)%len(alphabet)])
	}
	return b.String(), nil
}
