package main

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/containers/podman/v5/pkg/systemd/parser"
	"primamateria.systems/materia/pkg/components"
)

var ErrNoConfig = errors.New("no athanor config")

type ComponentBackupConfig struct {
	Volumes    map[string]QuadletBackupConfig
	Containers map[string]QuadletBackupConfig
}

type QuadletBackupConfig struct {
	Skip         bool   `toml:"skip"`
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
		default:
			continue
		}
	}
	return target, nil
}
