package merge

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gravitational/gamma/internal/action"
	"github.com/gravitational/gamma/internal/logger"
	"github.com/gravitational/gamma/internal/utils"
	"github.com/gravitational/gamma/internal/workspace"
)

var workingDirectory string
var workspaceManifest string
var actionNames []string
var actionsMap map[string]action.Action

// Get workspace action names and indexed map
func getActions(actions []action.Action) ([]string, map[string]action.Action) {
	var actionNames []string
	actionsMap := make(map[string]action.Action)
	for _, action := range actions {
		actionNames = append(actionNames, action.Name())
		actionsMap[action.Name()] = action
	}

	return actionNames, actionsMap
}

var Command = &cobra.Command{
	Use:   "merge action",
	Short: "Merge target action yaml to stdout",
	Long:  `Writes the merged action yaml for the action passed in to stdout.`,
	Run: func(_ *cobra.Command, args []string) {
		if len(args) != 1 || actionsMap[args[0]] == nil {
			allActions := strings.Join(actionNames, ", ")
			logger.Fatalf("Must specify exactly 1 target action to merge, choose from [%s]", allActions)
		}
		targetAction := actionsMap[args[0]]
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

	workingDirectory = utils.FetchWorkingDirectory(workingDirectory)
	wdArr, err := utils.NormalizeDirectories(workingDirectory)
	if err != nil {
		logger.Fatal(err)
	}
	ws := workspace.New(workspace.Properties{
		WorkingDirectory:  wdArr[0],
		WorkspaceManifest: workspaceManifest,
	})

	actions, err := ws.CollectActions()
	if err != nil {
		logger.Fatal(err)
	}
	actionNames, actionsMap = getActions(actions)

	Command.ValidArgsFunction = func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		return actionNames, cobra.ShellCompDirectiveDefault
	}
}
