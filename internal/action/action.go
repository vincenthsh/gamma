package action

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/gamma/internal/node"
	"github.com/gravitational/gamma/internal/schema"
	publicshema "github.com/gravitational/gamma/pkg/schema"
)

type Kind int

const (
	Javascript Kind = iota
	Composite
	Docker
)

type action struct {
	kind             Kind
	name             string
	packageInfo      *node.PackageInfo
	workspaceInfo    *publicshema.ActionConfig
	outputDirectory  string
	workingDirectory string
	owner            string
	repoName         string
}

type Config struct {
	Name             string
	WorkingDirectory string
	OutputDirectory  string
	PackageInfo      *node.PackageInfo
	WorkspaceInfo    *publicshema.ActionConfig
}

type Action interface {
	Build() error
	Name() string
	Version() string
	Path() string
	Owner() string
	RepoName() string
	OutputDirectory() string
	Contains(filename string) bool
}

func New(config *Config) (Action, error) {
	var uriString string
	var kind Kind

	switch {
	case config.PackageInfo != nil && config.PackageInfo.Repository != nil:
		kind = Javascript
		uriString = config.PackageInfo.Repository.URL
	case config.WorkspaceInfo != nil:
		kind = Composite
		uriString = config.WorkspaceInfo.RepositoryURL
	default:
		return nil, errors.New("repository field missing in Action")
	}

	uri, err := url.Parse(uriString)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(uri.Path[1:], "/")

	return &action{
		kind:             kind,
		name:             config.Name,
		packageInfo:      config.PackageInfo,
		workspaceInfo:    config.WorkspaceInfo,
		outputDirectory:  config.OutputDirectory,
		workingDirectory: config.WorkingDirectory,
		owner:            parts[0],
		repoName:         strings.TrimSuffix(parts[1], ".git"),
	}, nil
}

func (a *action) Name() string {
	switch a.kind {
	case Javascript:
		return a.packageInfo.Name
	default:
		return a.name
	}
}

func (a *action) Version() string {
	switch a.kind {
	case Javascript:
		return a.packageInfo.Version
	default:
		return a.workspaceInfo.Version
	}
}

func (a *action) Path() string {
	switch a.kind {
	case Javascript:
		return a.packageInfo.Path
	default:
		return a.workspaceInfo.OutputDirectory
	}
}

func (a *action) RepoName() string {
	return a.repoName
}

func (a *action) OutputDirectory() string {
	return a.outputDirectory
}

func (a *action) Owner() string {
	return a.owner
}

func (a *action) Contains(filename string) bool {
	normalizedPath, _ := filepath.Rel(a.workingDirectory, a.Path())

	return strings.HasPrefix(filename, normalizedPath+"/")
}

func (a *action) buildPackage() error {
	if a.kind != Javascript {
		return fmt.Errorf("action %s is not a Javascript action, can't build package", a.name)
	}
	cmd := exec.Command("pnpm", "exec", "nx", "run", fmt.Sprintf("%s:build", a.packageInfo.Name))
	cmd.Dir = a.packageInfo.Path

	if err := a.runCommand(cmd); err != nil {
		return err
	}

	return a.movePackage()
}

func (a *action) runCommand(cmd *exec.Cmd) error {
	var err error
	var relativePath string
	var workingDir string

	if workingDir, err = os.Getwd(); err != nil {
		return err
	}

	if relativePath, err = filepath.Rel(workingDir, a.Path()); err != nil {
		return err
	}

	cmd.Stderr = os.Stderr
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		fmt.Printf("⚡️%s: %s\n", relativePath, scanner.Text())
	}

	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func (a *action) movePackage() error {
	dist := path.Join(a.packageInfo.Path, "dist")
	destination := path.Join(a.outputDirectory, "dist")

	if err := os.Rename(dist, destination); err != nil {
		return err
	}

	return nil
}

func (a *action) createActionYAML() error {
	filename := path.Join(a.Path(), "action.yml")

	definition, err := schema.GetConfig(a.workingDirectory, filename)
	if err != nil {
		return err
	}

	bytes, err := yaml.Marshal(definition)
	if err != nil {
		return err
	}

	output := path.Join(a.outputDirectory, "action.yml")
	if err := os.WriteFile(output, bytes, 0644); err != nil {
		return fmt.Errorf("could not create action.yml: %v", err)
	}

	return nil
}

func (a *action) copyFile(file string) error {
	src := path.Join(a.Path(), file)
	dst := path.Join(a.outputDirectory, file)

	if _, err := os.Stat(src); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}

	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("could not create file: %v", err)
	}

	defer destination.Close()

	if _, err := io.Copy(destination, source); err != nil {
		return err
	}

	return nil
}

func (a *action) copyFiles() error {
	files := []string{
		"README.md",
	}

	var eg errgroup.Group

	for _, file := range files {
		f := file
		eg.Go(func() error {
			return a.copyFile(f)
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

func (a *action) createOutputDirectory() error {
	if err := os.Mkdir(a.outputDirectory, 0755); err != nil {
		return fmt.Errorf("could not create the output directory: %v", err)
	}

	return nil
}

func (a *action) Build() error {
	if err := a.createOutputDirectory(); err != nil {
		return fmt.Errorf("could not create output directory: %v", err)
	}

	var eg errgroup.Group

	eg.Go(a.createActionYAML)
	eg.Go(a.copyFiles)
	if a.kind == Javascript {
		eg.Go(a.buildPackage)
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}
