//go:build windows

package editor

import (
	"fmt"
	"os/exec"
)

// OpenDirectory 在 Windows 上使用资源管理器打开目录。
func OpenDirectory(dirPath string) error {
	cmd := exec.Command("explorer.exe", dirPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("打开目录失败: %w", err)
	}
	return nil
}
