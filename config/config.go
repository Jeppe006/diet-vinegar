// Package config implements types and routines to configure Vinegar.
package config

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/vinegarhq/vinegar/roblox"
	"github.com/vinegarhq/vinegar/roblox/bootstrapper"
	"github.com/vinegarhq/vinegar/splash"
	"github.com/vinegarhq/vinegar/util"
	"github.com/vinegarhq/vinegar/wine"
)

// LogoPath is set at build-time to set the logo icon path, which is
// used in [splash.Config] to set the icon path.
var LogoPath string

// Config is a representation of a Roblox binary Vinegar configuration.
type Binary struct {
	Channel       string        `toml:"channel"`
	Launcher      string        `toml:"launcher"`
	Renderer      string        `toml:"renderer"`
	DiscordRPC    bool          `toml:"discord_rpc"`
	ForcedVersion string        `toml:"forced_version"`
	Dxvk          bool          `toml:"dxvk"`
	FFlags        roblox.FFlags `toml:"fflags"`
	Env           Environment   `toml:"env"`
	ForcedGpu     string        `toml:"gpu"`
	GameMode      bool          `toml:"gamemode"`
}

// Config is a representation of the Vinegar configuration.
type Config struct {
	WineRoot          string      `toml:"wineroot"`
	DxvkVersion       string      `toml:"dxvk_version"`
	MultipleInstances bool        `toml:"multiple_instances"`
	SanitizeEnv       bool        `toml:"sanitize_env"`
	Player            Binary      `toml:"player"`
	Studio            Binary      `toml:"studio"`
	Env               Environment `toml:"env"`

	Splash splash.Config `toml:"splash"`
}

var (
	ErrNeedDXVKRenderer = errors.New("dxvk is only valid with d3d renderers")
	ErrWineRootAbs      = errors.New("wine root path is not an absolute path")
	ErrWineRootInvalid  = errors.New("no wine binary present in wine root")
)

// Load will load the named file to a Config; if it doesn't exist, it
// will fallback to the default configuration.
//
// The returned configuration will always be appended ontop of the default
// configuration.
//
// Load is required for any initialization for Config, as it calls routines
// to setup certain variables and verifies the configuration.
func Load(name string) (Config, error) {
	cfg := Default()

	if _, err := os.Stat(name); errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(name, &cfg); err != nil {
		return cfg, err
	}

	return cfg, cfg.setup()
}

// Default returns a sane default configuration for Vinegar.
func Default() Config {
	return Config{
		DxvkVersion: "2.3",

		Env: Environment{
			"WINEARCH":                    "win64",
			"WINEDEBUG":                   "err-kerberos,err-ntlm",
			"WINEESYNC":                   "1",
			"WINEDLLOVERRIDES":            "dxdiagn,winemenubuilder.exe,mscoree,mshtml=",
			"DXVK_LOG_LEVEL":              "warn",
			"DXVK_LOG_PATH":               "none",
			"MESA_GL_VERSION_OVERRIDE":    "4.4",
			"__GL_THREADED_OPTIMIZATIONS": "1",
		},

		Player: Binary{
			Dxvk:       true,
			GameMode:   true,
			ForcedGpu:  "prime-discrete",
			Renderer:   "D3D11",
			Channel:    bootstrapper.DefaultChannel,
			DiscordRPC: true,
			FFlags: roblox.FFlags{
				"DFIntTaskSchedulerTargetFps": 640,
			},
			Env: Environment{
				"OBS_VKCAPTURE": "1",
			},
		},
		Studio: Binary{
			Dxvk:      true,
			GameMode:  true,
			Channel:   bootstrapper.DefaultChannel,
			ForcedGpu: "prime-discrete",
			Renderer:  "D3D11",
			// TODO: fill with studio fflag/env goodies
			FFlags: make(roblox.FFlags),
			Env:    make(Environment),
		},

		Splash: splash.Config{
			Enabled:     true,
			LogoPath:    LogoPath,
			BgColor:     0x242424,
			FgColor:     0xfafafa,
			CancelColor: 0xbc3c3c,
			TrackColor:  0x2b002e,
			AccentColor: 0xbc5eb3,
			InfoColor:   0x777777,
		},
	}
}

func (b *Binary) LauncherPath() (string, error) {
	return exec.LookPath(strings.Fields(b.Launcher)[0])
}

func (b *Binary) setup() error {
	if err := b.FFlags.SetRenderer(b.Renderer); err != nil {
		return err
	}

	if !strings.HasPrefix(b.Renderer, "D3D11") && b.Dxvk {
		return ErrNeedDXVKRenderer
	}

	if b.Launcher != "" {
		if _, err := b.LauncherPath(); err != nil {
			return err
		}
	}

	return b.pickCard()
}

func (c *Config) setup() error {
	if c.SanitizeEnv {
		util.SanitizeEnv()
	}

	if c.WineRoot != "" {
		bin := filepath.Join(c.WineRoot, "bin")

		if !filepath.IsAbs(c.WineRoot) {
			return ErrWineRootAbs
		}

		c.Env["PATH"] = bin + ":" + os.Getenv("PATH")
		os.Unsetenv("WINEDLLPATH")
	}

	c.Env.Setenv()

	// system wine handled by vinegar
	if c.WineRoot != "" && !wine.WineLook() {
		return ErrWineRootInvalid
	}

	if err := c.Player.setup(); err != nil {
		return fmt.Errorf("player: %w", err)
	}

	if err := c.Studio.setup(); err != nil {
		return fmt.Errorf("studio: %w", err)
	}

	return nil
}
