package main

import (
	"context"
	"fmt"
	"slices"

	athanor "git.saintnet.tech/stryan/athanor/internal"
	"primamateria.systems/materia/pkg/containers"
	"primamateria.systems/materia/pkg/plan"
	"primamateria.systems/materia/pkg/services"
)

type BackupPlan struct {
	Components map[string]*plan.Plan
}

func NewBackupPlan() *BackupPlan {
	return &BackupPlan{
		Components: make(map[string]*plan.Plan),
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

func buildPlan(ctx context.Context, compMgr *athanor.Reader, conman containers.ContainerManager, serv services.ServiceManager, name, group string) (*BackupPlan, error) {
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
	fullPlan := NewBackupPlan()
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

	return fullPlan, nil
}

func printBackupPlan(bp *BackupPlan, format string) error {
	if len(bp.Components) == 0 {
		fmt.Println("no changes made")
		return nil
	}
	for _, c := range bp.Keys() {
		p := bp.Components[c]
		err := printPlan(p, format)
		if err != nil {
			return err
		}

	}

	return nil
}

func printPlan(p *plan.Plan, format string) error {
	if p.Empty() {
		fmt.Println("No changes made")
		return nil
	}
	switch format {
	case "text":
		fmt.Println(p.Pretty())
	case "json":
		jsonPlan, err := p.ToJson()
		if err != nil {
			return fmt.Errorf("error converting to json: %w", err)
		}
		fmt.Printf("%s", string(jsonPlan))
	default:
		return fmt.Errorf("unsupported output format: %q", format)
	}
	return nil
}
