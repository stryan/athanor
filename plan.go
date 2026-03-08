package main

import (
	"context"
	"fmt"
	"slices"

	"primamateria.systems/materia/pkg/containers"
	"primamateria.systems/materia/pkg/plan"
	"primamateria.systems/materia/pkg/services"
)

func buildPlan(ctx context.Context, compMgr *reader, conman containers.ContainerManager, serv services.ServiceManager, name string) (*plan.Plan, error) {
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

	p := plan.NewPlan()
	for _, cname := range compNames {
		c, err := loadComponent(ctx, conman, compMgr, cname)
		if err != nil {
			return nil, err
		}
		steps, err := PlanComponentBackup(ctx, serv, c)
		if err != nil {
			return nil, err
		}
		if err = p.Append(steps); err != nil {
			return nil, err
		}
	}

	return p, nil
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
