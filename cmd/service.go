package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"kairo/internal/config"
)

const unitTemplate = `[Unit]
Description=Kairo personal AI agent
PartOf=graphical-session.target
After=graphical-session.target pipewire.service wireplumber.service
Wants=pipewire.service

[Service]
Type=simple
ExecStart=%s start
Restart=on-failure
RestartSec=5
TimeoutStopSec=15
Environment="PATH=%s"

[Install]
WantedBy=graphical-session.target
`

func unitPath() string {
	return config.Expand("~/.config/systemd/user/kairo.service")
}

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Run Kairo automatically after login (systemd user service)",
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install and start the systemd user service",
	RunE: func(cmd *cobra.Command, args []string) error {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(unitPath()), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(unitPath(), []byte(fmt.Sprintf(unitTemplate, exe, os.Getenv("PATH"))), 0o644); err != nil {
			return err
		}
		steps := [][]string{
			{"systemctl", "--user", "import-environment", "WAYLAND_DISPLAY", "HYPRLAND_INSTANCE_SIGNATURE", "XDG_RUNTIME_DIR", "DISPLAY", "PATH"},
			{"systemctl", "--user", "daemon-reload"},
			{"systemctl", "--user", "enable", "--now", "kairo.service"},
		}
		for _, s := range steps {
			if out, err := exec.Command(s[0], s[1:]...).CombinedOutput(); err != nil {
				return fmt.Errorf("%v: %s", s, out)
			}
		}
		fmt.Println("installed", unitPath())
		fmt.Println("logs: journalctl --user -u kairo -f")
		return nil
	},
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Stop and remove the systemd user service",
	RunE: func(cmd *cobra.Command, args []string) error {
		exec.Command("systemctl", "--user", "disable", "--now", "kairo.service").Run()
		os.Remove(unitPath())
		exec.Command("systemctl", "--user", "daemon-reload").Run()
		fmt.Println("removed", unitPath())
		return nil
	},
}

func init() {
	serviceCmd.AddCommand(serviceInstallCmd, serviceUninstallCmd)
	rootCmd.AddCommand(serviceCmd)
}
