package uds

import (
	"fmt"
	"strings"

	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
)

// buildPreTaskCommands builds a shell command prefix that runs pre-tasks in order.
// Returns an empty string if there are no pre-tasks.
// Each task produces: "uds run -f <name> --set K1=V1 --set K2=V2"
// Tasks are joined with " && ".
func buildPreTaskCommands(preTasks []udsv1alpha3.RunnerPreTask) string {
	if len(preTasks) == 0 {
		return ""
	}

	parts := make([]string, 0, len(preTasks))
	for _, task := range preTasks {
		cmd := "uds run -f " + task.Name
		for key, value := range task.Variables {
			cmd += fmt.Sprintf(" --set %s=%s", key, value)
		}
		parts = append(parts, cmd)
	}

	return strings.Join(parts, " && ")
}
