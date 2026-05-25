//go:build linux

package editor

import (
	"fmt"
	"os/exec"
)

// OpenDirectory 在 Linux 上使用 xdg-open 打开目录。
func OpenDirectory(dirPath string) error {
	cmd := exec.Command("xdg-open", dirPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("打开目录失败: %w", err)
	}
	return nil
}
