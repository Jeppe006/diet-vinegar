package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/vinegarhq/aubun/bootstrapper"
	"github.com/vinegarhq/aubun/internal/config"
	"github.com/vinegarhq/aubun/internal/config/state"
	"github.com/vinegarhq/aubun/internal/dirs"
	"github.com/vinegarhq/aubun/wine"
	"github.com/vinegarhq/aubun/wine/dxvk"
)

func SetupBinary(ver bootstrapper.Version, dir string) {
	if err := dirs.Mkdir(dir); err != nil {
		log.Fatal(err)
	}

	if err := dirs.Mkdir(dirs.Downloads); err != nil {
		log.Fatal(err)
	}

	manifest, err := bootstrapper.FetchManifest(ver, dirs.Downloads)
	if err != nil {
		log.Fatal(err)
	}

	if err := manifest.Download(); err != nil {
		log.Fatal(err)
	}

	if err := manifest.Extract(dir, ver.Type.Directories()); err != nil {
		log.Fatal(err)
	}

	if err := state.SaveManifest(&manifest); err != nil {
		log.Fatal(err)
	}

	if err := state.CleanPackages(); err != nil {
		log.Fatal(err)
	}

	if err := state.CleanVersions(); err != nil {
		log.Fatal(err)
	}
}

func Binary(bin bootstrapper.BinaryType, cfg *config.Config, pfx *wine.Prefix, args ...string) {
	var appCfg config.Application
	var exe string

	switch bin {
	case bootstrapper.Player:
		appCfg = cfg.Player
		exe = "RobloxPlayerBeta.exe"
	case bootstrapper.Studio:
		appCfg = cfg.Player
		exe = "RobloxStudioBeta.exe"
	default:
		log.Fatal("invalid binary type given")
	}

	if appCfg.Dxvk {
		dxvkInstalled, err := state.DxvkInstalled()
		if err != nil {
			log.Fatal(err)
		}

		if !dxvkInstalled {
			if err := dxvk.Fetch(dirs.Cache); err != nil {
				log.Fatal(err)
			}

			if err := dxvk.Extract(dirs.Cache, pfx); err != nil {
				log.Fatal(err)
			}

			if err := state.SaveDxvk(true); err != nil {
				log.Fatal(err)
			}
		}

		dxvk.Setenv()
	} else if !cfg.Player.Dxvk && !cfg.Studio.Dxvk {
		if err := dxvk.Remove(pfx); err != nil {
			log.Fatal(err)
		}

		if err := state.SaveDxvk(false); err != nil {
			log.Fatal(err)
		}
	}

	channel := appCfg.Channel
	if strings.HasPrefix(strings.Join(args, " "), "roblox-player:1") {
		args, channel = bootstrapper.ParsePlayerURI(args[0])
	}

	if channel != appCfg.Channel {
		log.Printf("WARNING: Launch URI has a different channel: %s, forcing user-specified channel", channel)
		channel = appCfg.Channel
	}

	ver, err := bootstrapper.LatestVersion(bin, channel)
	if err != nil {
		log.Fatal(err)
	}
	verDir := filepath.Join(dirs.Versions, ver.GUID)

	_, err = os.Stat(filepath.Join(verDir, "AppSettings.xml"))
	if err != nil {
		log.Printf("Updating/Installing %s", bin.String())
		SetupBinary(ver, verDir)
	} else {
		log.Printf("%s is up to date (%s)", bin.String(), ver.GUID)
	}

	log.Printf("Launching %s", bin.String())

	args = append([]string{filepath.Join(verDir, exe)}, args...)

	if err := pfx.Exec(args...); err != nil {
		log.Fatal(err)
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg := config.Load()
	cfg.Setenv()

	pfx := wine.New(dirs.Prefix, "")
	if err := pfx.Setup(); err != nil {
		log.Fatal(err)
	}

	switch os.Args[1] {
	case "player":
		Binary(bootstrapper.Player, &cfg, &pfx, os.Args[2:]...)
	case "studio":
		Binary(bootstrapper.Studio, &cfg, &pfx, os.Args[2:]...)
	case "exec":
		if err := pfx.Exec(os.Args[2:]...); err != nil {
			log.Fatal(err)
		}
	}
}
