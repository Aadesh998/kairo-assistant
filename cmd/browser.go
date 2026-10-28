package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"kairo/internal/config"
)

var browserDataDirs = map[string]string{
	"brave":    "~/.config/BraveSoftware/Brave-Browser",
	"chrome":   "~/.config/google-chrome",
	"chromium": "~/.config/chromium",
}

var skipDirs = map[string]bool{
	"Cache": true, "Code Cache": true, "GPUCache": true, "Service Worker": true,
	"DawnCache": true, "DawnGraphiteCache": true, "DawnWebGPUCache": true, "GrShaderCache": true, "ShaderCache": true,
}

var browserCmd = &cobra.Command{
	Use:   "browser",
	Short: "The browser Kairo uses to open links and join meetings",
}

var browserImportCmd = &cobra.Command{
	Use:   "import [brave|chrome|chromium] [profile name]",
	Short: "Copy one of your browser profiles (with its logins) for Kairo, e.g. kairo browser import brave imagine.bo",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		base, ok := browserDataDirs[args[0]]
		if !ok {
			return fmt.Errorf("unknown browser %q, use brave, chrome or chromium", args[0])
		}
		base = config.Expand(base)
		profile, err := findProfile(base, args[1])
		if err != nil {
			return err
		}
		dest := config.Path(args[0] + "-" + strings.ReplaceAll(args[1], " ", "-"))
		os.RemoveAll(dest)
		if err := copyTree(filepath.Join(base, profile), filepath.Join(dest, "Default")); err != nil {
			return err
		}
		if err := copyFile(filepath.Join(base, "Local State"), filepath.Join(dest, "Local State")); err != nil {
			return err
		}
		fmt.Printf("copied %s profile %q (%s) to %s\n\nSet this in %s and restart Kairo:\n\nbrowser:\n  bin: %s\n  profile_dir: %s\n  password_store: gnome-libsecret\n",
			args[0], args[1], profile, dest, config.Path("config.yaml"), args[0], dest)
		return nil
	},
}

func findProfile(base, name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(base, "Local State"))
	if err != nil {
		return "", err
	}
	var state struct {
		Profile struct {
			InfoCache map[string]struct {
				Name string `json:"name"`
			} `json:"info_cache"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return "", err
	}
	var names []string
	for dir, info := range state.Profile.InfoCache {
		if strings.EqualFold(info.Name, name) {
			return dir, nil
		}
		names = append(names, info.Name)
	}
	return "", fmt.Errorf("no profile named %q, found: %s", name, strings.Join(names, ", "))
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(src, path)
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o700)
		}
		if !info.Mode().IsRegular() || strings.HasPrefix(info.Name(), "Singleton") {
			return nil
		}
		return copyFile(path, filepath.Join(dst, rel))
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func init() {
	browserCmd.AddCommand(browserImportCmd)
	rootCmd.AddCommand(browserCmd)
}
