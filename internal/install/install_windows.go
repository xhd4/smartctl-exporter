//go:build windows

package install

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
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
	binPath := fmt.Sprintf(`"%s" --config.file="%s"`, hostDest, configDest)

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
		fmt.Sprintf("binPath: %s", binPath),
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

	if err := createService(binPath); err != nil {
		return err
	}
	_ = configureRecovery()
	if err := runSC(fmt.Sprintf("start %s", ServiceName)); err != nil {
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
	sid, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return err
	}
	isMember, err := token.IsMember(sid)
	if err != nil {
		return err
	}
	if !isMember {
		return fmt.Errorf("Administrator privileges are required. Run elevated")
	}
	return nil
}

func stopService() {
	_ = runSC(fmt.Sprintf("stop %s", ServiceName))
	time.Sleep(2 * time.Second)
	_ = runSC(fmt.Sprintf("delete %s", ServiceName))
	time.Sleep(1 * time.Second)
}

func createService(binPath string) error {
	args := fmt.Sprintf(`create %s binPath= %s start= auto DisplayName= "%s"`, ServiceName, binPath, DisplayName)
	if err := runSC(args); err != nil {
		return err
	}
	_ = runSC(fmt.Sprintf(`description %s "%s"`, ServiceName, Description))
	return nil
}

func configureRecovery() error {
	_ = runSC(fmt.Sprintf("failure %s reset= 86400 actions= restart/60000/restart/60000/restart/60000", ServiceName))
	_ = runSC(fmt.Sprintf("failureflag %s 1", ServiceName))
	return nil
}

func runSC(arguments string) error {
	cmd := exec.Command("cmd.exe", "/c", "sc.exe "+arguments)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sc %s: %w: %s", arguments, err, strings.TrimSpace(string(out)))
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
