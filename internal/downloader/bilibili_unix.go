//go:build !windows

package downloader

import "os/exec"

func setSysProcAttr(cmd *exec.Cmd) {
	// 在非 Windows 平台上，不需要设置 SysProcAttr
}
