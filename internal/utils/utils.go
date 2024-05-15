package utils

import (
	"fmt"
	"os"
	"path"

	"github.com/gravitational/gamma/internal/logger"
)

func NormalizeDirectories(directories ...string) ([]string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("could not get current working directory: %v", err)
	}
	normalizedDirectories := make([]string, len(directories))
	for i, directory := range directories {
		if directory == "" {
			directory = wd
		} else {
			if !path.IsAbs(directory) {
				directory = path.Join(wd, directory)
			}
		}
		normalizedDirectories[i] = directory
	}
	return normalizedDirectories, nil
}

func FetchWorkingDirectory(dir string) string {
	if dir != "the current working directory" { // this is the default value from the flag
		return dir
	}

	wd, err := os.Getwd()
	if err != nil {
		logger.Fatalf("could not get current working directory: %v", err)
	}

	return wd
}
