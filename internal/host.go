package athanor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"primamateria.systems/materia/pkg/actions"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/containers"
	"primamateria.systems/materia/pkg/services"
)

func NewHostComponent(path string) (*components.Component, error) {
	hostComp := components.NewComponent("host")
	hostComp.State = components.StateRoot
	quadletEntries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read host path: %w", err)
	}
	for _, v := range quadletEntries {
		if v.IsDir() {
			continue
		}
		if strings.HasSuffix(v.Name(), ".volume") || strings.HasSuffix(v.Name(), ".container") || strings.HasSuffix(v.Name(), ".pod") {
			quadlet := components.Resource{
				Path:   v.Name(),
				Parent: "host",
				Kind:   components.FindResourceType(v.Name()),
			}
			rawQuadletContent, err := os.ReadFile(filepath.Join(path, v.Name()))
			if err != nil {
				return nil, fmt.Errorf("unable to read host resource %v: %w", v.Name(), err)
			}
			quadlet.Content = string(rawQuadletContent)
			hostObject, err := quadlet.GetHostObject(quadlet.Content)
			if err != nil {
				return nil, fmt.Errorf("unable to get host object for host resource %v: %w", v.Name(), err)
			}
			quadlet.HostObject = hostObject

			err = hostComp.Resources.Add(quadlet)
			if err != nil {
				return nil, fmt.Errorf("unable to add resource %v to host component: %w", v.Name(), err)
			}
		}
	}
	return hostComp, nil
}

func PlanHostBackup(ctx context.Context, conman containers.ContainerManager, serv services.ServiceManager, c *components.Component, group string) ([]actions.Action, error) {
	steps := make([]actions.Action, 0)
	cfgs, err := parseQuadlets(ctx, c)
	if err != nil {
		return steps, err
	}
	err = addHostParents(ctx, c, conman, cfgs)
	if err != nil {
		return steps, fmt.Errorf("couldn't add parents: %w", err)
	}
	if group != "" {
		for key, cfg := range cfgs {
			if cfg.Group == group {
				delete(cfgs, key)
			}
		}
	}

	steps, err = processConfigs(ctx, c, conman, serv, cfgs)
	if err != nil {
		return steps, err
	}
	return steps, nil
}

func addHostParents(ctx context.Context, c *components.Component, conman containers.ContainerManager, cfgs map[components.Resource]*QuadletBackupConfig) error {
	for v, cfg := range cfgs {
		if v.Kind != components.ResourceTypeVolume {
			continue
		}
		if cfg.Skip || cfg.InPlace {
			continue
		}
		parents, err := findParents(c, v)
		if err != nil {
			return err
		}
		// if len(parents) > 0 {
		for _, p := range parents {
			if _, ok := cfgs[p]; !ok {
				cfgs[p] = &QuadletBackupConfig{}
			}
		}
		//}  else {
		// 	parents, err := findNonQuadletParents(ctx, conman, v)
		// 	if err != nil {
		// 		return err
		// 	}
		// 	for _, p := range parents {
		// 		if _, ok := cfgs[p]; !ok {
		// 			cfgs[p] = &QuadletBackupConfig{}
		// 		}
		// 	}
		// }
	}
	return nil
}

// TODO bring back once materia supports non-quadlet containers better
// func findNonQuadletParents(ctx context.Context, conman containers.ContainerManager, vol components.Resource) ([]components.Resource, error) {
// 	result := make([]components.Resource, 0)
// 	runningContainers, err := conman.ListContainers(ctx, containers.ContainerListFilter{
// 		Volume: vol.Name(),
// 	})
// 	if err != nil {
// 		return result, err
// 	}
// 	for _, rc := range runningContainers {
// 		result = append(result, components.Resource{
// 			Path:       rc.Name,
// 			HostObject: rc.Name,
// 			Parent:     "host",
// 			Kind:       components.ResourceTypeContainer,
// 		})
// 	}
// 	return result, nil
// }
