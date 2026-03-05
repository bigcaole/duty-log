package utils

import (
	"strings"

	"duty-log-system/internal/crypto"
)

const backupPasswordCipherPrefix = "enc:"

func EncryptBackupPassword(secretKey, plaintext string) (string, error) {
	trimmed := strings.TrimSpace(plaintext)
	if trimmed == "" {
		return "", nil
	}
	key := crypto.DeriveAES256Key(secretKey)
	ciphertext, err := crypto.EncryptAES256GCM(key, trimmed)
	if err != nil {
		return "", err
	}
	return backupPasswordCipherPrefix + ciphertext, nil
}

func DecryptBackupPassword(secretKey, stored string) (string, error) {
	trimmed := strings.TrimSpace(stored)
	if trimmed == "" {
		return "", nil
	}
	if !strings.HasPrefix(trimmed, backupPasswordCipherPrefix) {
		return trimmed, nil
	}

	raw := strings.TrimPrefix(trimmed, backupPasswordCipherPrefix)
	key := crypto.DeriveAES256Key(secretKey)
	plaintext, err := crypto.DecryptAES256GCM(key, raw)
	if err != nil {
		return "", err
	}
	return plaintext, nil
}

// ResolveBackupPassword decrypts the stored backup password for display/use.
// It also returns a normalized encrypted value that can be persisted when
// legacy plaintext data is detected.
func ResolveBackupPassword(secretKey, stored string) (plaintext string, normalizedStored string, err error) {
	trimmed := strings.TrimSpace(stored)
	if trimmed == "" {
		return "", "", nil
	}
	if strings.HasPrefix(trimmed, backupPasswordCipherPrefix) {
		plain, decErr := DecryptBackupPassword(secretKey, trimmed)
		return plain, trimmed, decErr
	}

	encrypted, encErr := EncryptBackupPassword(secretKey, trimmed)
	if encErr != nil {
		return trimmed, trimmed, encErr
	}
	return trimmed, encrypted, nil
}
