package main

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	athanor "git.saintnet.tech/stryan/athanor/internal"
	"primamateria.systems/materia/pkg/containers"
	"primamateria.systems/materia/pkg/plan"
	"primamateria.systems/materia/pkg/services"
)

type BackupPlan struct {
	Components     map[string]*plan.Plan `json:"components"`
	OutputLocation string
}

func NewBackupPlan(cfg *athanor.Config) *BackupPlan {
	return &BackupPlan{
		Components:     make(map[string]*plan.Plan),
		OutputLocation: cfg.OutputDir,
	}
}

func (bp *BackupPlan) Keys() []string {
	keys := make([]string, 0, len(bp.Components))
	for k := range bp.Components {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func (bp *BackupPlan) ToJson() ([]byte, error) {
	return json.Marshal(bp)
}

func buildPlan(ctx context.Context, cfg *athanor.Config, compMgr *athanor.Reader, conman containers.ContainerManager, serv services.ServiceManager, name, group string) (*BackupPlan, error) {
	compNames, err := compMgr.ListComponentNames()
	if err != nil {
		return nil, err
	}

	if name != "" {
		if slices.Contains(compNames, name) {
			compNames = []string{name}
		} else {
			return nil, fmt.Errorf("component not found: %v", name)
		}
	}
	fullPlan := NewBackupPlan(cfg)
	for _, cname := range compNames {
		p := plan.NewPlan()
		c, err := athanor.LoadComponent(ctx, conman, compMgr, cname)
		if err != nil {
			return nil, err
		}
		steps, err := athanor.PlanComponentBackup(ctx, serv, c, group)
		if err != nil {
			return nil, err
		}
		if err = p.Append(steps); err != nil {
			return nil, err
		}
		fullPlan.Components[cname] = p
	}
	if cfg.HostMode {
		p := plan.NewPlan()
		c, err := athanor.NewHostComponent(cfg.QuadletDir)
		if err != nil {
			return nil, err
		}
		steps, err := athanor.PlanHostBackup(ctx, conman, serv, c, group)
		if err != nil {
			return nil, err
		}
		if err = p.Append(steps); err != nil {
			return nil, err
		}
		fullPlan.Components["host"] = p
	}

	return fullPlan, nil
}

func printBackupPlan(bp *BackupPlan, format string) error {
	if len(bp.Components) == 0 {
		fmt.Println("no changes made")
		return nil
	}
	switch format {
	case "text":
		fmt.Println("Backup Plan:")
		fmt.Printf("Backing up to: %v\n", bp.OutputLocation)
		for _, c := range bp.Keys() {
			p := bp.Components[c]
			fmt.Printf("%v Plan:\n", c)
			fmt.Printf("%v\n", p.Pretty())
		}
	case "json":
		jsonPlan, err := bp.ToJson()
		if err != nil {
			return fmt.Errorf("error converting to json: %w", err)
		}
		fmt.Printf("%s", string(jsonPlan))
	default:
		return fmt.Errorf("unsupported output format: %q", format)
	}

	return nil
}
