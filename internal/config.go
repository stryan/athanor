package athanor

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

func GetDefaultConfig() (*Config, error) {
	currentUser, err := user.Current()
	if err != nil {
		return nil, err
	}
	quadletPath := "/etc/containers/systemd/"
	outputPath := "/var/backups"
	dataPath := "/var/lib/materia/components/"
	if currentUser.Uid != "0" {
		home := currentUser.HomeDir
		var found bool
		conf, found := os.LookupEnv("XDG_CONFIG_HOME")
		if !found {
			quadletPath = filepath.Join(home, ".config", "containers", "systemd")
		} else {
			quadletPath = filepath.Join(conf, "containers", "systemd")
		}
		datadir, found := os.LookupEnv("XDG_DATA_HOME")
		if !found {
			dataPath = filepath.Join(home, ".local", "share")
		} else {
			dataPath = datadir
		}

		outputPath = filepath.Join(dataPath, "backups")
		dataPath = filepath.Join(dataPath, "materia", "components")
	}
	return &Config{
		QuadletDir: quadletPath,
		OutputDir:  outputPath,
		DataDir:    dataPath,
		HostMode:   false,
	}, nil
}

type Config struct {
	QuadletDir         string `toml:"quadlet_dir" koanf:"quadlet_dir"`
	OutputDir          string `toml:"output_dir" koanf:"output_dir"`
	DataDir            string `toml:"data_dir" koanf:"data_dir"`
	CompressionCommand string `toml:"compression_command" koanf:"compression_command"`
	CompressionSuffix  string `toml:"compression_suffix" koanf:"compression_suffix"`
	HostMode           bool   `toml:"host_mode" koanf:"host_mode"`
	PostCommand        string `toml:"post_command" koanf:"post_command"`
	Notify             bool   `toml:"notify" koanf:"notify"`
	Webhook            string `toml:"webhook" koanf:"webhook"`
}

func NewConfig(filename string) (*Config, error) {
	k := koanf.New(".")
	defaultConf, err := GetDefaultConfig()
	if err != nil {
		return nil, err
	}
	err = k.Load(structs.Provider(defaultConf, "koanf"), nil)
	if err != nil {
		return nil, err
	}
	err = k.Load(env.Provider("ATHANOR_", ".", func(s string) string {
		return strings.ToLower(
			strings.TrimPrefix(s, "ATHANOR_"))
	}), nil)
	if err != nil {
		return nil, err
	}
	if filename != "" {
		err = k.Load(file.Provider(filename), toml.Parser())
		if err != nil {
			return nil, err
		}
	}
	var c Config
	if err = k.Unmarshal("", &c); err != nil {
		return nil, err
	}
	if c.CompressionSuffix == "" {
		switch c.CompressionCommand {
		case "zstd":
			c.CompressionSuffix = "zstd"
		case "gzip":
			c.CompressionSuffix = "gz"
		case "zip":
			c.CompressionSuffix = "zip"
		default:
			c.CompressionSuffix = "compressed"
		}
	}

	return &c, nil
}

func (c *Config) String() string {
	result := fmt.Sprintf("Config:\nQuadlet Dir: %v\nData Dir: %v\nOutput Dir: %v", c.QuadletDir, c.DataDir, c.OutputDir)
	if c.CompressionCommand != "" {
		result += fmt.Sprintf("\nCompression Command: %v\n", c.CompressionCommand)
		result += fmt.Sprintf("Compression Suffix: %v", c.CompressionSuffix)
	} else {
		result += "\nCompression Disabled"
	}
	return result
}
