package utils

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"duty-log-system/internal/config"
)

func RestoreDatabaseBackup(ctx context.Context, appConfig config.AppConfig, backupFilePath, password string, clean bool) error {
	if strings.TrimSpace(backupFilePath) == "" {
		return fmt.Errorf("备份文件路径不能为空")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	sourcePath := filepath.Clean(backupFilePath)
	ext := strings.ToLower(filepath.Ext(sourcePath))

	var cleanupPaths []string
	defer func() {
		for _, p := range cleanupPaths {
			_ = os.Remove(p)
		}
	}()

	zipPath := ""
	sqlPath := ""

	switch ext {
	case ".sql":
		sqlPath = sourcePath
	case ".zip":
		zipPath = sourcePath
	case ".enc":
		if strings.TrimSpace(password) == "" {
			return fmt.Errorf("解密密码不能为空")
		}
		var err error
		zipPath, err = DecryptBackupBase64FileToTemp(sourcePath, password)
		if err != nil {
			return err
		}
		cleanupPaths = append(cleanupPaths, zipPath)
	default:
		return fmt.Errorf("不支持的备份文件类型，仅支持 .enc/.zip/.sql")
	}

	if sqlPath == "" {
		var err error
		sqlPath, err = extractSQLFromZip(zipPath)
		if err != nil {
			return err
		}
		cleanupPaths = append(cleanupPaths, sqlPath)
	}

	if clean {
		if err := resetPublicSchema(ctx, appConfig); err != nil {
			return err
		}
	}

	if err := runPSQLFile(ctx, appConfig, sqlPath); err != nil {
		return err
	}
	return nil
}

func extractSQLFromZip(zipPath string) (string, error) {
	if strings.TrimSpace(zipPath) == "" {
		return "", fmt.Errorf("zip 文件路径为空")
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var target *zip.File
	for _, f := range reader.File {
		if f == nil || f.FileInfo().IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f.Name), ".sql") {
			target = f
			break
		}
		if target == nil {
			target = f
		}
	}
	if target == nil {
		return "", fmt.Errorf("zip 内未找到备份文件")
	}

	src, err := target.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	tempFile, err := os.CreateTemp("", "duty-log-restore-*.sql")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	if err := tempFile.Chmod(0o600); err != nil {
		_ = os.Remove(tempFile.Name())
		return "", err
	}
	if _, err := io.Copy(tempFile, src); err != nil {
		_ = os.Remove(tempFile.Name())
		return "", err
	}
	return tempFile.Name(), nil
}

func runPSQLFile(ctx context.Context, appConfig config.AppConfig, sqlPath string) error {
	if strings.TrimSpace(sqlPath) == "" {
		return fmt.Errorf("SQL 文件路径为空")
	}
	cmd := exec.CommandContext(
		ctx,
		"psql",
		"-h", appConfig.DBHost,
		"-p", appConfig.DBPort,
		"-U", appConfig.DBUser,
		"-d", appConfig.DBName,
		"-v", "ON_ERROR_STOP=1",
		"-f", sqlPath,
	)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+appConfig.DBPassword)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("psql 导入失败: %w, output: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func resetPublicSchema(ctx context.Context, appConfig config.AppConfig) error {
	cmd := exec.CommandContext(
		ctx,
		"psql",
		"-h", appConfig.DBHost,
		"-p", appConfig.DBPort,
		"-U", appConfig.DBUser,
		"-d", appConfig.DBName,
		"-v", "ON_ERROR_STOP=1",
		"-c", "DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public;",
	)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+appConfig.DBPassword)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("清理数据库失败: %w, output: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
