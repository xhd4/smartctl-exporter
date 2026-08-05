//go:build windows

package install

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
	"smartctl-exporter/internal/config"
	"smartctl-exporter/internal/firewall"
	"smartctl-exporter/internal/smartmontools"
)

const (
	ServiceName   = "smartctl-exporter"
	DisplayName   = "Smartctl Exporter"
	Description   = "Prometheus smartctl exporter (S.M.A.R.T. metrics)"
	AppFolderName = "smartctl-exporter"
	HostExeName   = "smartctl-exporter.exe"
)

func InstallDir() string {
	return filepath.Join(os.Getenv("ProgramFiles"), AppFolderName)
}

func ProgramDataDir() string {
	return filepath.Join(os.Getenv("ProgramData"), AppFolderName)
}

func Install(cfg config.Config, dryRun bool) error {
	if err := ensureAdmin(); err != nil {
		return err
	}

	sourceHost, err := os.Executable()
	if err != nil {
		return err
	}

	installDir := InstallDir()
	hostDest := filepath.Join(installDir, HostExeName)
	configDest := filepath.Join(installDir, "config.yaml")
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cwdConfig := filepath.Join(cwd, "config.yaml")
	logDir := filepath.Join(ProgramDataDir(), "logs")

	port, err := cfg.ListenPort()
	if err != nil {
		return err
	}

	cwdAction := "(create new)"
	if _, err := os.Stat(cwdConfig); err == nil {
		cwdAction = "(keep existing)"
	}
	pfAction := "(copy from cwd)"
	if _, err := os.Stat(configDest); err == nil {
		pfAction = "(keep existing)"
	}

	printPlan("Install plan", dryRun, []string{
		fmt.Sprintf("Copy: %s -> %s", sourceHost, hostDest),
		fmt.Sprintf("Config (cwd): %s %s", cwdConfig, cwdAction),
		fmt.Sprintf("Config (install): %s %s", configDest, pfAction),
		fmt.Sprintf("Logs: %s", logDir),
		fmt.Sprintf("smartctl: %s (auto-install if missing)", cfg.SmartctlPath),
		fmt.Sprintf("Firewall enabled=%v profile=%s port=%d", cfg.FirewallEnabled, cfg.FirewallProfile, port),
		fmt.Sprintf("Service: %s", ServiceName),
		fmt.Sprintf("binPath: %s", hostDest),
		fmt.Sprintf("Metrics: http://localhost:%d%s", port, cfg.TelemetryPath),
	})
	if dryRun {
		return nil
	}

	if err := smartmontools.Ensure(cfg.SmartctlPath, cfg.InstallerVersion); err != nil {
		return fmt.Errorf("smartmontools: %w", err)
	}

	if err := os.MkdirAll(installDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	stopService()
	if err := copyFile(sourceHost, hostDest); err != nil {
		return err
	}

	if _, err := os.Stat(cwdConfig); os.IsNotExist(err) {
		writeCfg := cfg
		writeCfg.LogFile = filepath.Join(logDir, "exporter.log")
		yaml, err := writeCfg.SerializeYAML()
		if err != nil {
			return err
		}
		if err := os.WriteFile(cwdConfig, []byte(yaml), 0644); err != nil {
			return err
		}
	}

	if _, err := os.Stat(configDest); os.IsNotExist(err) {
		if err := copyFile(cwdConfig, configDest); err != nil {
			return err
		}
	}

	_ = firewall.DeleteAll()
	if cfg.FirewallEnabled {
		if err := firewall.Ensure(true, cfg.FirewallProfile, hostDest, port); err != nil {
			return fmt.Errorf("firewall: %w", err)
		}
	}

	if err := createAndStartService(hostDest); err != nil {
		return err
	}

	fmt.Printf("[OK] Installed and started: %s\n", ServiceName)
	fmt.Printf("[OK] Metrics: http://localhost:%d%s\n", port, cfg.TelemetryPath)
	return nil
}

func Uninstall(dryRun bool) error {
	if err := ensureAdmin(); err != nil {
		return err
	}
	printPlan("Uninstall plan", dryRun, []string{
		fmt.Sprintf("Stop/delete service: %s", ServiceName),
		"Delete firewall rules (smartctl-exporter / SmartctlExporter)",
		fmt.Sprintf("Remove: %s", InstallDir()),
		fmt.Sprintf("Remove: %s", ProgramDataDir()),
	})
	if dryRun {
		return nil
	}

	stopService()
	_ = firewall.DeleteAll()
	killByName("smartctl-exporter")
	_ = os.RemoveAll(InstallDir())
	_ = os.RemoveAll(ProgramDataDir())
	fmt.Println("[OK] Uninstalled")
	return nil
}

func printPlan(title string, dryRun bool, lines []string) {
	suffix := ""
	if dryRun {
		suffix = " (dry-run)"
	}
	fmt.Printf("=== %s%s ===\n", title, suffix)
	for _, line := range lines {
		fmt.Printf("  %s\n", line)
	}
}

func ensureAdmin() error {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return err
	}
	defer token.Close()
	if !token.IsElevated() {
		return fmt.Errorf("Administrator privileges are required. Run elevated")
	}
	return nil
}

func stopService() {
	m, err := mgr.Connect()
	if err != nil {
		return
	}
	defer m.Disconnect()

	s, err := m.OpenService(ServiceName)
	if err != nil {
		return
	}
	defer s.Close()

	_, _ = s.Control(svc.Stop)
	time.Sleep(2 * time.Second)
	_ = s.Delete()
	time.Sleep(1 * time.Second)
}

func createAndStartService(exePath string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.CreateService(ServiceName, exePath, mgr.Config{
		DisplayName: DisplayName,
		Description: Description,
		StartType:   mgr.StartAutomatic,
	})
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer s.Close()

	_ = s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 60 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 60 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 60 * time.Second},
	}, 86400)
	_ = s.SetRecoveryActionsOnNonCrashFailures(true)

	if err := s.Start(); err != nil {
		return fmt.Errorf("start service: %w", err)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func killByName(name string) {
	cmd := exec.Command("taskkill", "/F", "/IM", name+".exe")
	_ = cmd.Run()
}
