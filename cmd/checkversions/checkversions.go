package checkversions

import (
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"

	"github.com/gravitational/gamma/internal/action"
	"github.com/gravitational/gamma/internal/git"
	"github.com/gravitational/gamma/internal/logger"
	"github.com/gravitational/gamma/internal/utils"
	"github.com/gravitational/gamma/internal/workspace"
)

var workingDirectory string
var workspaceManifest string

var Command = &cobra.Command{
	Use:   "check-versions",
	Short: "Check versions of changed actions in the monorepo",
	Long:  `Finds all changed actions and verifies their current version has no existing tag.`,
	Run: func(_ *cobra.Command, _ []string) {
		started := time.Now()

		workingDirectory = utils.FetchWorkingDirectory(workingDirectory)
		wda, err := utils.NormalizeDirectories(workingDirectory)
		if err != nil {
			logger.Fatal(err)
		}

		repo, err := git.New(wda[0])
		if err != nil {
			logger.Fatal(err)
		}

		logger.Info("collecting changed files")

		changed, err := repo.GetChangedFiles()
		if err != nil {
			logger.Fatal(err)
		}

		logger.Infof("files changed [%s]", strings.Join(changed, ", "))

		ws := workspace.New(workspace.Properties{
			WorkingDirectory:  wda[0],
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

		var actionsToVerify []action.Action

	outer:
		for _, action := range actions {
			for _, file := range changed {
				if action.Contains(file) {
					actionsToVerify = append(actionsToVerify, action)

					continue outer
				}
			}
		}
		if len(actionsToVerify) == 0 {
			logger.Warning("no actions have changed, exiting")

			return
		}

		var hasError bool

		for _, action := range actionsToVerify {
			logger.Infof("action %s has changes, verifying version", action.Name())

			verifyStarted := time.Now()

			if exists, err := repo.TagExists(action); err != nil || exists {
				hasError = true
				if err != nil {
					logger.Errorf("error verifying action %s: %v", action.Name(), err)
				}
				if exists {
					logger.Errorf("version %s@v%s already exists", action.Name(), action.Version())
					continue
				}
			}

			verifyTook := time.Since(verifyStarted)

			logger.Successf("successfully verified action %s@v%s in %.2fs", action.Name(), action.Version(), verifyTook.Seconds())
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
	Command.Flags().StringVarP(&workingDirectory, "directory", "d", "the current working directory", "directory containing the monorepo of actions")
	Command.Flags().StringVarP(&workspaceManifest, "workspace", "w", "gamma-workspace.yml", "workspace manifest for non-javascript actions")
}
