package build

import (
	"os"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"

	"github.com/gravitational/gamma/internal/logger"
	"github.com/gravitational/gamma/internal/utils"
	"github.com/gravitational/gamma/internal/workspace"
)

var outputDirectory string
var workingDirectory string
var workspaceManifest string

var Command = &cobra.Command{
	Use:   "build",
	Short: "Builds all the actions in the monorepo",
	Long:  `Builds all the actions in the monorepo and puts them into the specified output directory, separated by repo.`,
	Run: func(cmd *cobra.Command, args []string) {
		started := time.Now()

		workingDirectory = utils.FetchWorkingDirectory(workingDirectory)

		nd, err := utils.NormalizeDirectories(workingDirectory, outputDirectory)
		if err != nil {
			logger.Fatal(err)
		}
		wd, od := nd[0], nd[1]

		if err := os.RemoveAll(od); err != nil {
			logger.Fatalf("could not remove output directory: %v", err)
		}

		if err := os.Mkdir(od, 0755); err != nil {
			logger.Fatalf("could not create output directory: %v", err)
		}

		ws := workspace.New(workspace.Properties{
			WorkingDirectory:  wd,
			OutputDirectory:   od,
			WorkspaceManifest: workspaceManifest,
		})

		logger.Info("collecting actions")

		actions, err := ws.CollectActions()
		if err != nil {
			logger.Fatal(err)
		}

		if len(actions) == 0 {
			logger.Fatal("could not find any actions")
		}

		var actionNames []string
		for _, action := range actions {
			actionNames = append(actionNames, action.Name())
		}

		logger.Infof("found actions [%s]", strings.Join(actionNames, ", "))

		var hasError bool

		for _, action := range actions {
			logger.Infof("building action %s", action.Name())

			buildStarted := time.Now()

			if err := action.Build(); err != nil {
				hasError = true
				logger.Errorf("error building action %s: %v", action.Name(), err)

				continue
			}

			buildTook := time.Since(buildStarted)

			logger.Successf("successfully built action %s in %.2fs", action.Name(), buildTook.Seconds())
		}

		bold := text.Colors{text.FgWhite, text.Bold}

		took := time.Since(started)

		if hasError {
			logger.Fatal(bold.Sprintf("completed with errors in %.2fs", took.Seconds()))
		}

		logger.Success(bold.Sprintf("done in %.2fs", took.Seconds()))
	},
}

func init() {
	Command.Flags().StringVarP(&outputDirectory, "output", "o", "build", "output directory")
	Command.Flags().StringVarP(&workingDirectory, "directory", "d", "the current working directory", "directory containing the monorepo of actions")
	Command.Flags().StringVarP(&workspaceManifest, "workspace", "w", "gamma-workspace.yml", "workspace manifest for non-javascript actions")
}
