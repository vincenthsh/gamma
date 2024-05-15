package merge

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gravitational/gamma/internal/action"
	"github.com/gravitational/gamma/internal/logger"
	"github.com/gravitational/gamma/internal/utils"
	"github.com/gravitational/gamma/internal/workspace"
)

const outputDirectory = "fake"

var workingDirectory string
var workspaceManifest string

var Command = &cobra.Command{
	Use:   "merge action",
	Short: "Merge target action to stdout",
	Long:  `Writes the merged action yaml for the action passed in to stdout.`,
	Run: func(_ *cobra.Command, args []string) {
		if workingDirectory == "the current working directory" { // this is the default value from the flag
			wd, err := os.Getwd()
			if err != nil {
				logger.Fatalf("could not get current working directory: %v", err)
			}

			workingDirectory = wd
		}

		wd, od, err := utils.NormalizeDirectories(workingDirectory, outputDirectory)
		if err != nil {
			logger.Fatal(err)
		}

		ws := workspace.New(wd, od, workspaceManifest)

		actions, err := ws.CollectActions()
		if err != nil {
			logger.Fatal(err)
		}

		if len(actions) == 0 {
			logger.Fatal("could not find any actions")
		}

		var actionNames []string
		var targetAction action.Action
		for _, action := range actions {
			actionNames = append(actionNames, action.Name())
			if len(args) == 1 && action.Name() == args[0] {
				targetAction = action
			}
		}
		allActions := strings.Join(actionNames, ", ")
		if len(args) != 1 {
			logger.Fatalf("Must specify exectly 1 target action, choose from [%s]", allActions)
		} else if len(args) == 1 && targetAction == nil {
			logger.Fatalf("Target action %v not found in [%s]", args[0], allActions)
		}

		s, err := targetAction.GetActionYAML()
		if err != nil {
			logger.Errorf("error merging action %s: %v", targetAction.Name(), err)
		}
		fmt.Println(*s)
	},
}

func init() {
	Command.Flags().StringVarP(&workingDirectory, "directory", "d", "the current working directory", "directory containing the monorepo of actions")
	Command.Flags().StringVarP(&workspaceManifest, "workspace", "w", "gamma-workspace.yml", "workspace manifest for non-javascript actions")
}
