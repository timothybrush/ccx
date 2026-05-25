//go:build darwin

package editor

import (
	"fmt"
	"os/exec"
)

// OpenDirectory 在 macOS 上使用 Finder 打开目录。
func OpenDirectory(dirPath string) error {
	cmd := exec.Command("/usr/bin/open", dirPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("打开目录失败: %w", err)
	}
	return nil
}
