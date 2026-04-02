package athanor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"charm.land/log/v2"
	"github.com/containers/podman/v5/pkg/systemd/parser"
	"primamateria.systems/materia/pkg/actions"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/containers"
	"primamateria.systems/materia/pkg/loader"
	"primamateria.systems/materia/pkg/manifests"
	"primamateria.systems/materia/pkg/services"
)

var athanorGroup = "X-Athanor"

func PlanComponentBackup(ctx context.Context, conman containers.ContainerManager, serv services.ServiceManager, c *components.Component, group string) ([]actions.Action, error) {
	steps := make([]actions.Action, 0)
	cfgs, err := parseQuadlets(ctx, c)
	if err != nil {
		return steps, err
	}
	backupManifest, err := parseManifest(ctx, c)
	if err != nil {
		return steps, err
	}
	for key, o := range backupManifest.Volumes {
		res, err := c.Resources.Get(fmt.Sprintf("%v.volume", key))
		if err != nil {
			return steps, err
		}
		cfgs[res] = &o
	}
	err = addParents(ctx, c, cfgs)
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
	if backupManifest.PostCommand != "" {
		res := ParsePostCommand(c.Name, backupManifest.PostCommand)
		act := actions.Action{
			Todo:     actions.ActionStart,
			Parent:   c,
			Target:   res,
			Priority: 5,
		}
		if res.Kind == components.ResourceTypeScript {
			act.Todo = actions.ActionExecute
		}
		steps = append(steps, act)
	}
	return steps, nil
}

func LoadComponent(ctx context.Context, conMgr containers.ContainerManager, compMgr *Reader, compName string) (*components.Component, error) {
	pipeline := loader.NewHostComponentPipeline(compMgr, conMgr)
	comp := components.NewComponent(compName)
	err := pipeline.Load(ctx, comp)
	if err != nil {
		return nil, err
	}

	return comp, nil
}

func parseQuadlets(ctx context.Context, c *components.Component) (map[components.Resource]*QuadletBackupConfig, error) {
	configs := make(map[components.Resource]*QuadletBackupConfig)
	for _, res := range c.Resources.List() {
		if res.Kind == components.ResourceTypeContainer || res.Kind == components.ResourceTypeVolume || res.Kind == components.ResourceTypePod {
			cfg, err := ParseUnit(res)
			if err != nil && !errors.Is(err, ErrNoConfig) {
				return nil, err
			}
			if errors.Is(err, ErrNoConfig) {
				if res.Kind == components.ResourceTypeVolume {
					cfg = &QuadletBackupConfig{}
				} else {
					continue
				}
			}
			configs[res] = cfg
		}
	}
	return configs, nil
}

func addParents(ctx context.Context, c *components.Component, cfgs map[components.Resource]*QuadletBackupConfig) error {
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
		for _, p := range parents {
			if _, ok := cfgs[p]; !ok {
				cfgs[p] = &QuadletBackupConfig{}
			}
		}
	}
	return nil
}

func parseManifest(ctx context.Context, c *components.Component) (*ComponentBackupConfig, error) {
	manifestResource, err := c.Resources.Get(manifests.ComponentManifestFile)
	if err != nil {
		return nil, err
	}
	return ParseManifest(manifestResource)
}

func processConfigs(ctx context.Context, c *components.Component, conman containers.ContainerManager, serv services.ServiceManager, configs map[components.Resource]*QuadletBackupConfig) ([]actions.Action, error) {
	var steps []actions.Action
	var volumes []string
	for res, cfg := range configs {
		if cfg.Skip {
			continue
		}
		switch res.Kind {
		case components.ResourceTypeContainer, components.ResourceTypePod:
			serviceState, err := serv.GetService(ctx, res.Service())
			if err != nil {
				return steps, err
			}
			if cfg.DumpCommand != "" {
				if serviceState.State == "inactive" {
					// start up an inactive container
					waitState := "active"
					steps = append(steps, actions.Action{
						Todo:     actions.ActionStart,
						Parent:   c,
						Target:   res,
						Priority: 1,
						Metadata: &actions.ActionMetadata{
							ServiceUntilState: &waitState,
						},
					})
					endState := "inactive"
					steps = append(steps, actions.Action{
						Todo:     actions.ActionStop,
						Parent:   c,
						Target:   res,
						Priority: 2,
						Metadata: &actions.ActionMetadata{
							ServiceUntilState: &endState,
						},
					})
				}
				steps = append(steps, actions.Action{
					Todo:     actions.ActionExecute,
					Parent:   c,
					Target:   res,
					Priority: 1,
					Metadata: &actions.ActionMetadata{
						Command: &cfg.DumpCommand,
					},
				})
			}
			if cfg.InPlace {
				continue
			}
			if serviceState.State == "active" || serviceState.State == "activating" {
				waitState := "inactive"
				steps = append(steps, actions.Action{
					Todo:     actions.ActionStop,
					Parent:   c,
					Target:   res,
					Priority: 2,
					Metadata: &actions.ActionMetadata{
						ServiceUntilState: &waitState,
					},
				})
				endState := "active"
				steps = append(steps, actions.Action{
					Todo:     actions.ActionStart,
					Parent:   c,
					Target:   res,
					Priority: 4,
					Metadata: &actions.ActionMetadata{
						ServiceUntilState: &endState,
					},
				})
			}

		case components.ResourceTypeVolume:
			volType, err := res.QueryQuadletData("Type")
			if !errors.Is(err, components.ErrQuadletNoGroup) { // a volume quadlet with no content is valid
				if err != nil && !errors.Is(err, components.ErrQuadletNoKey) {
					return steps, fmt.Errorf("unable to query quadlet %v data: %w", res.Name(), err)
				}
				if !errors.Is(err, components.ErrQuadletNoKey) && volType[0] == "nfs" {
					log.Warnf("skipping backup of NFS volume %v", res.Name())
					continue
				}
				volDriver, err := res.QueryQuadletData("Driver")
				if err != nil && !errors.Is(err, components.ErrQuadletNoKey) {
					return steps, fmt.Errorf("unable to query quadlet %v data: %w", res.Name(), err)
				}
				if !errors.Is(err, components.ErrQuadletNoKey) && volDriver[0] != "local" {
					log.Warnf("skipping backup of non-local volume %v: %v", res.Name(), volDriver[0])
					continue
				}
			}

			_, err = conman.GetVolume(ctx, res.HostObject)
			if errors.Is(err, containers.ErrPodmanObjectNotFound) {
				log.Warn("skipping backing up non-existent volume %v", res.HostObject)
				continue
			}
			if err != nil && !errors.Is(err, containers.ErrPodmanObjectNotFound) {
				return steps, fmt.Errorf("unable to get volume %v %w", res.HostObject, err)
			}
			dumpCommand := actions.Action{
				Todo:     actions.ActionDump,
				Parent:   c,
				Target:   res,
				Priority: 3,
			}
			if cfg.BackupScript != "" {
				dumpScript, err := c.Resources.Get(cfg.BackupScript)
				if err != nil {
					return steps, err
				}
				dumpCommand = actions.Action{
					Todo:     actions.ActionExecute,
					Parent:   c,
					Target:   dumpScript,
					Priority: 3,
				}
			}
			volumes = append(volumes, res.Name())
			steps = append(steps, dumpCommand)

		}
	}
	if len(volumes) == 0 {
		// If we're not actually backing up anything, don't start/stop contaienrs
		return []actions.Action{}, nil
	}
	return steps, nil
}

func findParents(c *components.Component, vol components.Resource) ([]components.Resource, error) {
	result := []components.Resource{}
	for _, r := range c.Resources.List() {
		if r.Kind == components.ResourceTypeContainer || r.Kind == components.ResourceTypePod {
			unitfile := parser.NewUnitFile()
			err := unitfile.Parse(r.Content)
			if err != nil {
				return result, fmt.Errorf("error parsing systemd unit file: %w", err)
			}
			groupName := "Container"
			if r.Kind == components.ResourceTypePod {
				groupName = "Pod"
			}
			volKeys := unitfile.LookupAll(groupName, "Volume")
			for _, key := range volKeys {
				splitkey := strings.Split(key, ":")
				if len(splitkey) < 1 {
					return result, errors.New("invalid volume map")
				}
				volname := splitkey[0]
				if volname == vol.Name() {
					result = append(result, r)
				}
			}
		}
	}
	return result, nil
}
