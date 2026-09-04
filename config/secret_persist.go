package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// encryptMarkedConfigFiles 将当前环境配置中的 ENC(...) 加密并原子回写。
func encryptMarkedConfigFiles(configPath, env string, cipher *SecretCipher) error {
	paths, err := newEnvironmentFilePaths(configPath, env)
	if err != nil {
		return err
	}

	for _, path := range paths {
		var data []byte
		data, err = os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read config file %q: %w", path, err)
		}
		if !bytes.Contains(data, []byte(secretMarkerPrefix)) {
			continue
		}

		var encrypted []byte
		encrypted, err = encryptMarkedConfig(data, cipher)
		if err != nil {
			return fmt.Errorf("encrypt config file %q: %w", path, err)
		}
		if bytes.Equal(data, encrypted) {
			continue
		}

		var info os.FileInfo
		info, err = os.Stat(path)
		if err != nil {
			return fmt.Errorf("stat config file %q: %w", path, err)
		}
		if err = rewriteConfigFile(path, encrypted, info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

// encryptMarkedConfig 只替换 ENC(...) 标记，保留原配置文件的注释和排版。
func encryptMarkedConfig(data []byte, cipher *SecretCipher) ([]byte, error) {
	encrypted, _, err := cipher.encryptMarkedValue(string(data))
	if err != nil {
		return nil, err
	}
	return []byte(encrypted), nil
}

// rewriteConfigFile 使用临时文件原子替换配置，并保留原文件权限。
func rewriteConfigFile(path string, data []byte, mode os.FileMode) (err error) {
	tempFile, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create config temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if err = tempFile.Chmod(mode); err != nil {
		closeErr := tempFile.Close()
		return errors.Join(fmt.Errorf("chmod config temp file: %w", err), closeErr)
	}
	if _, err = tempFile.Write(data); err != nil {
		closeErr := tempFile.Close()
		return errors.Join(fmt.Errorf("write config temp file: %w", err), closeErr)
	}
	if err = tempFile.Sync(); err != nil {
		closeErr := tempFile.Close()
		return errors.Join(fmt.Errorf("sync config temp file: %w", err), closeErr)
	}
	if err = tempFile.Close(); err != nil {
		return fmt.Errorf("close config temp file: %w", err)
	}
	if err = os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace config file: %w", err)
	}
	return nil
}
