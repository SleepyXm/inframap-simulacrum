package simulation

import (
	"db-seeder/walker"
	"strings"
)

func findAuthEndpoints(ctx *walker.ProjectContext) (signup, login string) {
	for _, f := range ctx.Files {
		for _, ep := range f.Endpoints {
			path := strings.ToLower(ep.FullPath)
			if ep.Method == "POST" && strings.Contains(path, "signup") {
				signup = ep.FullPath
			}
			if ep.Method == "POST" && strings.Contains(path, "login") {
				login = ep.FullPath
			}
		}
	}
	return
}
