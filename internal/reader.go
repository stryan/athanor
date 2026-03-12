package athanor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/charmbracelet/log"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/manifests"
)

type Reader struct {
	QuadletPrefix string
	DataPrefix    string
}

func (r *Reader) GetComponent(name string) (*components.Component, error) {
	comp := components.NewComponent(name)
	dataPath := filepath.Join(r.DataPrefix, name)
	quadletPath := filepath.Join(r.QuadletPrefix, name)
	versionFileExists := true
	_, err := os.Stat(filepath.Join(dataPath, ".component_version"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			versionFileExists = false
		} else {
			return nil, fmt.Errorf("error reading component version: %w", err)
		}
	}
	if versionFileExists {
		k := koanf.New(".")
		err := k.Load(file.Provider(filepath.Join(dataPath, ".component_version")), toml.Parser())
		if err != nil {
			return nil, err
		}
		var c components.ComponentVersion
		err = k.Unmarshal("", &c)
		if err != nil {
			return nil, err
		}
		comp.Version = c.Version
	} else {
		comp.Version = -1
	}
	log.Debug("loading component", "component", comp.Name, "version", comp.Version)
	manifestPath := filepath.Join(dataPath, manifests.ComponentManifestFile)
	if _, err := os.Stat(manifestPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, components.ErrCorruptComponent
		}
		return nil, err
	}
	manifestResource, err := r.NewResource(comp, manifestPath)
	if err != nil {
		return nil, err
	}
	err = comp.Resources.Add(manifestResource)
	if err != nil {
		return nil, err
	}

	err = filepath.WalkDir(quadletPath, func(fullPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == comp.Name || d.Name() == ".materia_managed" {
			return nil
		}

		newRes, err := r.NewResource(comp, fullPath)
		if err != nil {
			return err
		}

		return comp.Resources.Add(newRes)
	})
	if err != nil {
		return nil, err
	}

	return comp, nil
}

func (r *Reader) GetResource(parent *components.Component, name string) (components.Resource, error) {
	if parent == nil || name == "" {
		return components.Resource{}, errors.New("invalid parent or resource")
	}
	quadletPath := filepath.Join(r.QuadletPrefix, parent.Name)
	resourcePath := filepath.Join(quadletPath, name)
	_, err := os.Stat(resourcePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return components.Resource{}, err
	} else if err != nil {
		return r.NewResource(parent, resourcePath)
	}
	return components.Resource{}, errors.New("resource not found")
}

func (r *Reader) GetManifest(parent *components.Component) (*manifests.ComponentManifest, error) {
	return manifests.LoadComponentManifestFromFile(filepath.Join(r.DataPrefix, parent.Name, manifests.ComponentManifestFile))
}

func (r *Reader) ReadResource(res components.Resource) (string, error) {
	resPath := ""
	if res.Kind == components.ResourceTypeDirectory {
		return "", nil
	}
	if res.Kind == components.ResourceTypePodmanSecret {
		return "", errors.New("secrets don't live in repositories")
	}
	if res.IsQuadlet() {
		resPath = filepath.Join(r.QuadletPrefix, res.Parent, res.Filepath())
	} else {
		resPath = filepath.Join(r.DataPrefix, res.Parent, res.Filepath())
	}

	curFile, err := os.ReadFile(resPath)
	if err != nil {
		return "", err
	}
	return string(curFile), nil
}

func (r *Reader) ListResources(c *components.Component) ([]components.Resource, error) {
	if c == nil {
		return []components.Resource{}, errors.New("invalid parent or resource")
	}
	resources := []components.Resource{}
	quadletPath := filepath.Join(r.QuadletPrefix, c.Name)
	searchFunc := func(prefix string) fs.WalkDirFunc {
		return func(fullPath string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.Name() == c.Name || d.Name() == ".component_version" || d.Name() == ".materia_managed" {
				return nil
			}
			resName, err := filepath.Rel(prefix, fullPath)
			if err != nil {
				return err
			}
			newRes := components.Resource{
				Parent:   c.Name,
				Path:     resName,
				Kind:     components.FindResourceType(resName),
				Template: components.IsTemplate(resName),
			}
			resources = append(resources, newRes)

			return nil
		}
	}
	err := filepath.WalkDir(quadletPath, searchFunc(quadletPath))
	if err != nil {
		return resources, err
	}
	return resources, nil
}

func (reader *Reader) ComponentExists(name string) (bool, error) {
	path := filepath.Join(reader.DataPrefix, name)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	checkpath := filepath.Join(reader.QuadletPrefix, name, ".materia_managed")
	_, err = os.Stat(checkpath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		} else {
			return false, fmt.Errorf("error checking if quadlet folder is materia managed: %w", err)
		}
	}
	return true, nil
}

func (r *Reader) ListComponentNames() ([]string, error) {
	var compPaths []string
	entries, err := os.ReadDir(r.QuadletPrefix)
	if err != nil {
		return nil, err
	}
	for _, v := range entries {
		if v.IsDir() {
			checkpath := filepath.Join(r.QuadletPrefix, v.Name(), ".materia_managed")
			_, err := os.Stat(checkpath)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				} else {
					return nil, fmt.Errorf("error checking if quadlet folder is materia managed: %w", err)
				}
			}
			compPaths = append(compPaths, v.Name())
		}
	}
	slices.Sort(compPaths)
	return compPaths, nil
}

func (reader *Reader) Clean() error {
	return nil
}

func (r *Reader) NewResource(parent *components.Component, path string) (components.Resource, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return components.Resource{}, err
	}
	res := components.Resource{
		Path:     path,
		Parent:   parent.Name,
		Template: false,
	}
	if fileInfo.IsDir() {
		res.Kind = components.ResourceTypeDirectory
	} else {
		res.Kind = components.FindResourceType(path)
	}
	if res.IsQuadlet() {
		res.Path, err = filepath.Rel(filepath.Join(r.QuadletPrefix, parent.Name), path)
		if err != nil {
			return res, err
		}
		unitData, err := r.ReadResource(res)
		if err != nil {
			return res, err
		}
		hostObject, err := res.GetHostObject(unitData)
		if err != nil {
			return res, err
		}
		res.HostObject = hostObject
	} else {
		res.Path, err = filepath.Rel(filepath.Join(r.DataPrefix, parent.Name), path)
		if err != nil {
			return res, err
		}
	}
	return res, nil
}
