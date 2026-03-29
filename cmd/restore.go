package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	athanor "git.saintnet.tech/stryan/athanor/internal"
	"github.com/charmbracelet/log"
	"primamateria.systems/materia/pkg/actions"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/plan"
	"primamateria.systems/materia/pkg/services"
)

func buildRestorePlan(ctx context.Context, targetComponent *components.Component, source string, serv *services.ServiceManager) (*plan.Plan, error) {
	volmap, err := GatherVolumes(targetComponent)
	if err != nil {
		return nil, fmt.Errorf("unable to gather component volumes for restore: %w", err)
	}
	log.Info("verifying volumes", "needed", volmap.ListNames())

	sourceEntries, err := os.ReadDir(source)
	if err != nil {
		return nil, err
	}
	for _, e := range sourceEntries {
		if sv := volmap.IsValidSource(e.Name()); sv != nil {
			err := volmap.SetSource(*sv, filepath.Join(source, e.Name()))
			if err != nil {
				if errors.Is(err, athanor.ErrExistingSource) {
					return nil, fmt.Errorf("multiple source canidates for volume %v with new %v: %w", sv.Name(), e.Name(), err)
				}
				return nil, err
			}
			log.Info("found source volume", "canidate", e.Name(), "target", sv.Name())
		}
	}
	missingVolumes := volmap.ListEmptyTargets()
	if len(missingVolumes) > 0 {
		return nil, fmt.Errorf("missing source volumes: %v", missingVolumes)
	}
	restorePlan := plan.NewPlan()
	addAction := func(a actions.Action) error {
		if err := restorePlan.Add(a); err != nil {
			return fmt.Errorf("unable to add %v action for %v: %w", a.Todo, a.Target.Name(), err)
		}
		return nil
	}
	// stop services
	needToStop, err := GatherServices(ctx, targetComponent, serv)
	if err != nil {
		return nil, fmt.Errorf("unable to gather services: %w", err)
	}
	for _, k := range needToStop {
		if err := addAction(actions.Action{
			Todo:     actions.ActionStop,
			Parent:   targetComponent,
			Target:   k,
			Priority: 1,
		}); err != nil {
			return nil, err
		}
		if err := addAction(actions.Action{
			Todo:     actions.ActionStart,
			Parent:   targetComponent,
			Target:   k,
			Priority: 4,
		}); err != nil {
			return nil, err
		}
	}

	// ensure volumes
	for _, vol := range volmap.ListVolumes() {
		if err := addAction(actions.Action{
			Todo:     actions.ActionEnsure,
			Parent:   targetComponent,
			Target:   vol,
			Priority: 2,
		}); err != nil {
			return nil, err
		}
	}

	// import volumes
	for _, vol := range volmap.ListVolumes() {
		src, err := volmap.GetSource(vol)
		if err != nil {
			return nil, err
		}
		if err := addAction(actions.Action{
			Todo:     actions.ActionImport,
			Parent:   targetComponent,
			Target:   vol,
			Priority: 3,
			Metadata: &actions.ActionMetadata{
				VolumeName: &src,
			},
		}); err != nil {
			return nil, err
		}
	}
	return restorePlan, nil
}

func GatherVolumes(targetComponent *components.Component) (*athanor.VolumeRestoreMap, error) {
	volmap := athanor.NewVolumeRestoreMap()
	for _, v := range targetComponent.Resources.List() {
		if v.Kind == components.ResourceTypeVolume {
			config, err := athanor.ParseUnit(v)
			if err != nil && !errors.Is(err, athanor.ErrNoConfig) {
				return nil, err
			}
			if !errors.Is(err, athanor.ErrNoConfig) && config.Skip {
				continue
			}
			volmap.AddTarget(v)
		}
	}
	return volmap, nil
}

func GatherServices(ctx context.Context, targetComponent *components.Component, serv *services.ServiceManager) ([]components.Resource, error) {
	resultSet := components.NewResourceSet()
	for _, src := range targetComponent.Services.List() {
		liveService, err := serv.GetService(ctx, src.Service)
		if err != nil {
			return []components.Resource{}, err
		}
		if liveService.State == "active" || liveService.State == "activating" {
			srcRes, err := targetComponent.Resources.Get(src.Service)
			if errors.Is(err, components.ErrResourceNotFound) {
				srcRes = components.Resource{
					Path:   src.Service,
					Parent: targetComponent.Name,
					Kind:   components.ResourceTypeService,
				}
			} else if err != nil {
				return []components.Resource{}, err
			}
			err = resultSet.Add(srcRes)
			if err != nil {
				return []components.Resource{}, err
			}
		}
	}
	for _, quadlet := range targetComponent.Resources.List() {
		if quadlet.Kind == components.ResourceTypeContainer || quadlet.Kind == components.ResourceTypePod {
			liveService, err := serv.GetService(ctx, quadlet.Service())
			if err != nil {
				return []components.Resource{}, err
			}
			if liveService.State == "active" || liveService.State == "activating" {
				err = resultSet.Add(quadlet)
				if err != nil {
					return []components.Resource{}, err
				}
			}
		}
	}
	return resultSet.List(), nil
}
