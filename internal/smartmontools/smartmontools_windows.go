//go:build windows

package smartmontools

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Ensure(smartctlPath, installerVersion string) error {
	if ok, _ := isWorking(smartctlPath); ok {
		return nil
	}

	if err := tryWinget(); err == nil {
		if ok, _ := isWorking(smartctlPath); ok {
			return nil
		}
		if ok, _ := isWorking(defaultPath()); ok {
			return nil
		}
	}

	if err := installFromGitHub(installerVersion); err != nil {
		return err
	}
	if ok, _ := isWorking(smartctlPath); ok {
		return nil
	}
	if ok, _ := isWorking(defaultPath()); ok {
		return nil
	}
	return fmt.Errorf("smartctl not found after install; expected %s", smartctlPath)
}

func defaultPath() string {
	return `C:\Program Files\smartmontools\bin\smartctl.exe`
}

func isWorking(path string) (bool, error) {
	if strings.TrimSpace(path) == "" {
		return false, nil
	}
	if _, err := os.Stat(path); err != nil {
		return false, err
	}
	cmd := exec.Command(path, "-V")
	if err := cmd.Run(); err != nil {
		return false, err
	}
	return true, nil
}

func tryWinget() error {
	winget, err := exec.LookPath("winget")
	if err != nil {
		return err
	}
	cmd := exec.Command(winget, "install", "--id", "Smartmontools.Smartmontools", "-e", "--silent",
		"--accept-package-agreements", "--accept-source-agreements")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("winget: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func installFromGitHub(version string) error {
	ver := strings.TrimSpace(version)
	if ver == "" {
		ver = "7.5"
	}
	ver = strings.TrimPrefix(ver, "v")
	tag := "RELEASE_" + strings.ReplaceAll(ver, ".", "_")
	asset := fmt.Sprintf("smartmontools-%s.win32-setup.exe", ver)
	url := fmt.Sprintf("https://github.com/smartmontools/smartmontools/releases/download/%s/%s", tag, asset)

	tmp := filepath.Join(os.TempDir(), asset)
	if err := download(url, tmp); err != nil {
		return err
	}
	defer os.Remove(tmp)

	cmd := exec.Command(tmp, "/S")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("smartmontools setup: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func download(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}
