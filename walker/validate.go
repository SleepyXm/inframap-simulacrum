package walker

import (
	"errors"
	"fmt"
	"strings"
)

func ValidateCaptureResult(result *CaptureResult) error {
	if result == nil {
		return errors.New("capture result is nil")
	}

	if len(result.Groups) == 0 {
		return errors.New("capture result contains no directory groups")
	}

	var problems []string
	seenFiles := map[string]bool{}

	for groupIndex, group := range result.Groups {
		if strings.TrimSpace(group.Dir) == "" {
			problems = append(problems, fmt.Sprintf("groups[%d] has empty dir", groupIndex))
		}

		for fileIndex, file := range group.Files {
			if strings.TrimSpace(file.Path) == "" {
				problems = append(problems, fmt.Sprintf("groups[%d].files[%d] has empty path", groupIndex, fileIndex))
				continue
			}

			if seenFiles[file.Path] {
				problems = append(problems, fmt.Sprintf("duplicate captured file: %s", file.Path))
			}

			seenFiles[file.Path] = true
		}
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "\n"))
	}

	return nil
}
