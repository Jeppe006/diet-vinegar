package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/vinegarhq/vinegar/config"
	"github.com/vinegarhq/vinegar/config/editor"
	"github.com/vinegarhq/vinegar/internal/dirs"
	"github.com/vinegarhq/vinegar/roblox"
	"github.com/vinegarhq/vinegar/sysinfo"
	"github.com/vinegarhq/vinegar/wine"
)

var (
	BinPrefix string
	Version   string
)

func usage() {
	fmt.Fprintln(os.Stderr, "usage: vinegar [-config filepath] player|studio [args...]")
	fmt.Fprintln(os.Stderr, "       vinegar [-config filepath] exec prog args...")
	fmt.Fprintln(os.Stderr, "       vinegar [-config filepath] kill|winetricks|sysinfo")
	fmt.Fprintln(os.Stderr, "       vinegar delete|edit|log|version")
	os.Exit(1)
}

func main() {
	configPath := flag.String("config", filepath.Join(dirs.Config, "config.toml"), "config.toml file which should be used")
	flag.Parse()

	cmd := flag.Arg(0)
	args := flag.Args()

	wine.Wine = "wine64"

	switch cmd {
	// These commands don't require a configuration
	case "delete", "edit", "submit", "version", "log":
		switch cmd {
		case "delete":
			Delete()
		case "edit":
			if err := editor.Edit(*configPath); err != nil {
				log.Fatal(err)
			}
		case "version":
			fmt.Println("Vinegar", Version)
		case "submit":
			fmt.Println("There is no merlin, silly!!")
		case "log":
			OpenLog()
		}
	// These commands (except player & studio) don't require a configuration,
	// but they require a wineprefix, hence wineroot of configuration is required.
	case "sysinfo", "player", "studio", "exec", "kill", "winetricks":
		cfg, err := config.Load(*configPath)
		if err != nil {
			log.Fatal(err)
		}

		pfx := wine.New(dirs.Prefix, os.Stderr)
		// Always ensure its created, wine will complain if the root
		// directory doesnt exist
		if err := os.MkdirAll(dirs.Prefix, 0o755); err != nil {
			log.Fatal(err)
		}

		switch cmd {
		case "sysinfo":
			Sysinfo(&pfx)
		case "exec":
			if len(args) < 2 {
				usage()
			}

			if err := pfx.Wine(args[1], args[2:]...).Run(); err != nil {
				log.Fatal(err)
			}
		case "kill":
			pfx.Kill()
		case "winetricks":
			if err := pfx.Winetricks(); err != nil {
				log.Fatal(err)
			}
		case "player":
			NewBinary(roblox.Player, &cfg, &pfx).Main(args[1:]...)
		case "studio":
			NewBinary(roblox.Studio, &cfg, &pfx).Main(args[1:]...)
		}
	default:
		usage()
	}
}

func Delete() {
	log.Println("Deleting Wineprefix")
	if err := os.RemoveAll(dirs.Prefix); err != nil {
		log.Fatal(err)
	}
}

func OpenLog() {
	dir := filepath.Join(dirs.Logs)

	files, err := os.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}

	var birthTime time.Time
	var names []string

	for _, entry := range files {
		fi, err := entry.Info()
		if err != nil {
			log.Fatal(err)
		}
		if fi.Mode().IsRegular() && filepath.Ext(fi.Name()) == ".log" {
			if !fi.ModTime().Before(birthTime) {
				if fi.ModTime().After(birthTime) {
					birthTime = fi.ModTime()
					names = names[:0]
				}
				names = append(names, fi.Name())
			}
		}
	}
	if len(names) > 0 {
		logFile := dir + "/" + strings.Join(names, "")
		editor.EditNonToml(logFile)
	} else {
		fmt.Println("No log files found.")
	}
}

func Sysinfo(pfx *wine.Prefix) {
	cmd := pfx.Wine("--version")
	cmd.Stdout = nil // required for Output()
	ver, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}

	var revision string
	bi, _ := debug.ReadBuildInfo()
	for _, bs := range bi.Settings {
		if bs.Key == "vcs.revision" {
			revision = fmt.Sprintf("(%s)", bs.Value)
		}
	}

	info := `* Vinegar: %s %s
* Distro: %s
* Processor: %s
  * Supports AVX: %t
  * Supports split lock detection: %t
* Kernel: %s
* Wine: %s`

	fmt.Printf(info, Version, revision, sysinfo.Distro, sysinfo.CPU.Name, sysinfo.CPU.AVX, sysinfo.CPU.SplitLockDetect, sysinfo.Kernel, ver)
	if sysinfo.InFlatpak {
		fmt.Println("* Flatpak: [x]")
	}

	fmt.Println("* Cards:")
	for i, c := range sysinfo.Cards {
		fmt.Printf("  * Card %d: %s [%s %s %s]\n", i+1, c.Name, c.Driver, path.Base(c.Device), c.Path)
	}
}

func LogFile(name string) (*os.File, error) {
	if err := dirs.Mkdirs(dirs.Logs); err != nil {
		return nil, err
	}

	// name-2006-01-02T15:04:05Z07:00.log
	path := filepath.Join(dirs.Logs, name+"-"+time.Now().Format(time.RFC3339)+".log")

	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s log file: %w", name, err)
	}

	log.Printf("Logging to file: %s", path)

	return file, nil
}
