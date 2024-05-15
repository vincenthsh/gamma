package workspace

import (
	"fmt"
	"os"
	"path"

	"github.com/gravitational/gamma/internal/action"
	"github.com/gravitational/gamma/internal/node"
	"github.com/gravitational/gamma/pkg/schema"
	"gopkg.in/yaml.v3"
)

type Workspace interface {
	CollectActions() ([]action.Action, error)
}

type workspace struct {
	workingDirectory  string
	outputDirectory   string
	workspaceManifest string
	packages          node.PackageService
}

type Properties struct {
	WorkingDirectory  string
	OutputDirectory   string
	WorkspaceManifest string
}

func New(props Properties) Workspace {
	if props.WorkingDirectory == "" {
		props.WorkingDirectory = "."
	}
	if props.OutputDirectory == "" {
		props.OutputDirectory = "build"
	}

	return &workspace{
		props.WorkingDirectory,
		props.OutputDirectory,
		props.WorkspaceManifest,
		node.NewPackageService(props.WorkingDirectory),
	}
}

func (w *workspace) CollectActions() ([]action.Action, error) {
	// TODO: don't error if rootPackage doesn't exist
	rootPackage, err := w.readRootPackage()
	if err != nil {
		return nil, err
	}

	nodeWorkspaces, err := w.packages.GetWorkspaces(rootPackage)
	if err != nil {
		return nil, err
	}

	var actions []action.Action
	for _, ws := range nodeWorkspaces {
		outputDirectory := path.Join(w.outputDirectory, ws.Name)

		config := &action.Config{
			Name:             ws.Name,
			WorkingDirectory: w.workingDirectory,
			OutputDirectory:  outputDirectory,
			PackageInfo:      ws,
		}

		action, err := action.New(config)
		if err != nil {
			return nil, err
		}

		actions = append(actions, action)
	}

	workspaceManifest, err := w.readWorkspaceManifest()
	if err != nil {
		return nil, err
	}

	if workspaceManifest != nil {
		for _, a := range workspaceManifest.Actions {
			outputDirectory := path.Join(w.outputDirectory, a.Name)

			// Create a new instance of 'a' that is scoped to this loop iteration.
			a := a
			config := &action.Config{
				Name:             a.Name,
				WorkingDirectory: w.workingDirectory,
				OutputDirectory:  outputDirectory,
				ActionInfo:       &a,
			}

			action, err := action.New(config)
			if err != nil {
				return nil, err
			}

			actions = append(actions, action)
		}
	}

	return actions, nil
}

func (w *workspace) readRootPackage() (*node.PackageInfo, error) {
	p := path.Join(w.workingDirectory, "package.json")

	return w.packages.ReadPackageInfo(p)
}

// readWorkspaceManifest if it exists
func (w *workspace) readWorkspaceManifest() (*schema.WorkspaceManifest, error) {
	file, err := os.ReadFile(path.Join(w.workingDirectory, w.workspaceManifest))
	if err != nil {
		// ignore if file doesn't exist
		return nil, nil
	}
	var config schema.WorkspaceManifest
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml: %v", err)
	}
	return &config, nil
}
