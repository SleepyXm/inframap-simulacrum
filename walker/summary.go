package walker

import (
	"db-seeder/tools"
	"fmt"
)

func summariseCaptureToLines(result *CaptureResult) []tools.SummaryLine {
	var lines []tools.SummaryLine

	files := 0
	functions := 0
	classes := 0
	routes := 0
	imports := 0
	assignments := 0
	conditionals := 0

	for _, group := range result.Groups {
		files += len(group.Files)

		for _, file := range group.Files {
			functions += len(file.Functions)
			classes += len(file.Classes)
			routes += len(file.Routes)
			imports += len(file.Imports)
			assignments += len(file.Assignments)
			conditionals += len(file.Conditionals)
		}
	}

	lines = append(lines, tools.SummaryLine{Label: "files captured", Value: fmt.Sprintf("%d", files)})
	lines = append(lines, tools.SummaryLine{Label: "functions", Value: fmt.Sprintf("%d", functions)})
	lines = append(lines, tools.SummaryLine{Label: "classes", Value: fmt.Sprintf("%d", classes)})
	lines = append(lines, tools.SummaryLine{Label: "routes", Value: fmt.Sprintf("%d", routes)})
	lines = append(lines, tools.SummaryLine{Label: "imports", Value: fmt.Sprintf("%d", imports)})
	lines = append(lines, tools.SummaryLine{Label: "assignments", Value: fmt.Sprintf("%d", assignments)})
	lines = append(lines, tools.SummaryLine{Label: "conditionals", Value: fmt.Sprintf("%d", conditionals)})

	return lines
}
