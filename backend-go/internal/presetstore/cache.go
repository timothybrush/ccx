package presetstore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	bundleFileName = "bundle.json"
	shaFileName    = "bundle.json.sha256"
)

// LoadCache 从缓存目录加载 bundle，并校验 sidecar SHA256。
func LoadCache(cacheDir string) (*PresetBundle, error) {
	if strings.TrimSpace(cacheDir) == "" {
		return nil, fmt.Errorf("[presetstore] cacheDir 不能为空")
	}

	bundlePath := filepath.Join(cacheDir, bundleFileName)
	shaPath := filepath.Join(cacheDir, shaFileName)

	bundleBytes, err := os.ReadFile(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("[presetstore] 读取缓存 bundle 失败: %w", err)
	}
	shaBytes, err := os.ReadFile(shaPath)
	if err != nil {
		return nil, fmt.Errorf("[presetstore] 读取缓存 SHA256 失败: %w", err)
	}

	expected := strings.TrimSpace(string(shaBytes))
	actual := sha256Hex(bundleBytes)
	if expected == "" || !strings.EqualFold(expected, actual) {
		return nil, fmt.Errorf("[presetstore] 缓存 SHA256 校验失败")
	}

	var bundle PresetBundle
	if err := json.Unmarshal(bundleBytes, &bundle); err != nil {
		return nil, fmt.Errorf("[presetstore] 解析缓存 bundle 失败: %w", err)
	}
	if err := Validate(&bundle); err != nil {
		return nil, fmt.Errorf("[presetstore] 校验缓存 bundle 失败: %w", err)
	}

	return &bundle, nil
}

// SaveCache 原子写入 bundle.json 与 SHA256 sidecar。
func SaveCache(cacheDir string, bundle *PresetBundle) error {
	if strings.TrimSpace(cacheDir) == "" {
		return fmt.Errorf("[presetstore] cacheDir 不能为空")
	}
	if bundle == nil {
		return fmt.Errorf("[presetstore] bundle 为 nil")
	}
	if err := Validate(bundle); err != nil {
		return fmt.Errorf("[presetstore] 写缓存前校验失败: %w", err)
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("[presetstore] 创建缓存目录失败: %w", err)
	}

	bundleBytes, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("[presetstore] 序列化缓存 bundle 失败: %w", err)
	}
	shaBytes := []byte(sha256Hex(bundleBytes) + "\n")

	bundlePath := filepath.Join(cacheDir, bundleFileName)
	shaPath := filepath.Join(cacheDir, shaFileName)
	if err := writeFileAtomically(bundlePath, bundleBytes, 0o644); err != nil {
		return err
	}
	if err := writeFileAtomically(shaPath, shaBytes, 0o644); err != nil {
		return err
	}
	return nil
}

func writeFileAtomically(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp.*")
	if err != nil {
		return fmt.Errorf("[presetstore] 创建临时文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("[presetstore] 写入临时文件失败: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		return fmt.Errorf("[presetstore] 设置临时文件权限失败: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("[presetstore] 刷新临时文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("[presetstore] 关闭临时文件失败: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("[presetstore] 原子替换缓存文件失败: %w", err)
	}
	cleanup = false
	return nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
