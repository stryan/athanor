package athanor

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/containers/podman/v5/pkg/systemd/parser"
	"primamateria.systems/materia/pkg/components"
)

var ErrNoConfig = errors.New("no athanor config")

type ComponentBackupConfig struct {
	Volumes     map[string]QuadletBackupConfig
	Containers  map[string]QuadletBackupConfig
	PostCommand string `toml:"PostCommand"`
}

type QuadletBackupConfig struct {
	Skip         bool   `toml:"skip"`
	InPlace      bool   `toml:"inplace"`
	Group        string `toml:"group"`
	DumpCommand  string `toml:"dump_command"`
	BackupScript string `toml:"backup_script"`
}

func ParseUnit(res components.Resource) (*QuadletBackupConfig, error) {
	unitfile := parser.NewUnitFile()
	err := unitfile.Parse(res.Content)
	if err != nil {
		return nil, fmt.Errorf("error parsing systemd unit file: %w", err)
	}
	target := &QuadletBackupConfig{}
	if !unitfile.HasGroup(athanorGroup) {
		return nil, ErrNoConfig
	}
	keys := unitfile.ListKeys(athanorGroup)
	for _, k := range keys {
		switch k {
		case "DumpCommand":
			if res.Kind == components.ResourceTypeContainer || res.Kind == components.ResourceTypePod {
				target.DumpCommand, _ = unitfile.Lookup(athanorGroup, "DumpCommand")
			}
		case "Skip":
			shouldSkip, _ := unitfile.Lookup(athanorGroup, "Skip")
			target.Skip, err = strconv.ParseBool(shouldSkip)
			if err != nil {
				return nil, err
			}
		case "Group":
			group, _ := unitfile.Lookup(athanorGroup, "Group")
			target.Group = group

		case "InPlace":
			group, _ := unitfile.Lookup(athanorGroup, "InPlace")
			target.Group = group
		default:
			continue
		}
	}
	return target, nil
}

func ParseManifest(manifestResource components.Resource) (*ComponentBackupConfig, error) {
	var maniCfg ComponentBackupConfig
	err := toml.Unmarshal([]byte(manifestResource.Content), &maniCfg)
	if err != nil {
		return nil, err
	}
	return &maniCfg, nil
}

func ParsePostCommand(parent, cmd string) components.Resource {
	if strings.HasSuffix(cmd, ".service") || strings.HasSuffix(cmd, ".target") {
		return components.Resource{
			Path:   cmd,
			Parent: parent,
			Kind:   components.ResourceTypeService,
		}
	}
	return components.Resource{
		Path:   cmd,
		Parent: parent,
		Kind:   components.ResourceTypeScript,
	}
}
