package athanor

import (
	"errors"
	"fmt"
	"strings"

	"github.com/emirpasic/gods/maps"
	"github.com/emirpasic/gods/maps/treemap"
	"primamateria.systems/materia/pkg/components"
)

var (
	ErrExistingSource = errors.New("existing source volume canidate")
	ErrNoTarget       = errors.New("target volume not found")
	ErrNoSource       = errors.New("source volume not set")
)

type VolumeRestoreMap struct {
	volmap maps.Map
}

func NewVolumeRestoreMap() *VolumeRestoreMap {
	return &VolumeRestoreMap{
		volmap: treemap.NewWith(func(a, b interface{}) int {
			vola := a.(components.Resource)
			volb := b.(components.Resource)
			return strings.Compare(vola.Name(), volb.Name())
		}),
	}
}

func (v *VolumeRestoreMap) AddTarget(res components.Resource) {
	v.volmap.Put(res, "")
}

func (v *VolumeRestoreMap) SetSource(res components.Resource, src string) error {
	if existing, ok := v.volmap.Get(res); ok && existing != "" {
		return fmt.Errorf("%w: %v", ErrExistingSource, existing.(string))
	} else if !ok {
		return ErrNoTarget
	}
	v.volmap.Put(res, src)
	return nil
}

func (v *VolumeRestoreMap) GetSource(res components.Resource) (string, error) {
	if src, ok := v.volmap.Get(res); !ok {
		return "", ErrNoTarget
	} else if src == "" {
		return "", ErrNoSource
	} else {
		return src.(string), nil
	}
}

func (v *VolumeRestoreMap) ListVolumes() []components.Resource {
	results := make([]components.Resource, 0, v.volmap.Size())
	for _, v := range v.volmap.Keys() {
		results = append(results, v.(components.Resource))
	}

	return results
}

func (v *VolumeRestoreMap) ListNames() []string {
	results := make([]string, 0, v.volmap.Size())
	for _, v := range v.volmap.Keys() {
		res := v.(components.Resource)
		results = append(results, res.Name())
	}

	return results
}

func (v *VolumeRestoreMap) Size() int {
	return v.volmap.Size()
}

func (v *VolumeRestoreMap) ListEmptyTargets() []string {
	results := make([]string, 0, v.volmap.Size())
	for _, vol := range v.volmap.Keys() {
		res := vol.(components.Resource)
		srcInt, _ := v.volmap.Get(res)
		src := srcInt.(string)
		if src == "" {
			results = append(results, res.Name())
		}
	}
	return results
}

func (v *VolumeRestoreMap) IsValidSource(src string) *components.Resource {
	for _, v := range v.volmap.Keys() {
		sv := v.(components.Resource)
		svname := strings.TrimSuffix(sv.Name(), ".volume")
		if strings.Contains(src, svname) {
			return &sv
		}
	}
	return nil
}
