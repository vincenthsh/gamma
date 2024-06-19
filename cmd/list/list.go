package list

import (
	"time"

	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"

	"github.com/gravitational/gamma/internal/logger"
	"github.com/gravitational/gamma/internal/utils"
	"github.com/gravitational/gamma/internal/workspace"
)

var workingDirectory string
var workspaceManifest string

var Command = &cobra.Command{
	Use:   "list",
	Short: "List all the actions in the monorepo",
	Long:  `List all the actions in the monorepo.`,
	Run: func(_ *cobra.Command, _ []string) {
		started := time.Now()

		workingDirectory = utils.FetchWorkingDirectory(workingDirectory)

		nd, err := utils.NormalizeDirectories(workingDirectory)
		if err != nil {
			logger.Fatal(err)
		}

		ws := workspace.New(workspace.Properties{
			WorkingDirectory:  nd[0],
			WorkspaceManifest: workspaceManifest,
		})

		logger.Info("collecting actions")

		actions, err := ws.CollectActions(true)
		if err != nil {
			logger.Fatal(err)
		}

		if len(actions) == 0 {
			logger.Fatal("could not find any actions")
		}

		logger.Info("found actions:")
		for _, action := range actions {
			logger.Infof(" ✅ %s (%s/%s)", action.Name(), action.Owner(), action.RepoName())
		}

		took := time.Since(started)

		bold := text.Colors{text.FgWhite, text.Bold}
		logger.Success(bold.Sprintf("done in %.2fs", took.Seconds()))
	},
}

func init() {
	Command.Flags().StringVarP(&workingDirectory, "directory", "d", "the current working directory", "directory containing the monorepo of actions")
	Command.Flags().StringVarP(&workspaceManifest, "workspace", "w", "gamma-workspace.yml", "workspace manifest for non-javascript actions")
}
