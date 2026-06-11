package walker

import (
	"db-seeder/walker/code-diagnostic/routes"
	"db-seeder/walker/config"
	"db-seeder/walker/core/scanner"
	"db-seeder/walker/core/worker"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
)

type CaptureOptions struct {
	ConfigPath string
	RulesDir   string
	TargetDir  string
}

type DirGroup struct {
	Dir   string                `json:"dir" yaml:"dir"`
	Files []worker.FileSnapshot `json:"files" yaml:"files"`
}

type LanguageSummary struct {
	Name      string `json:"name" yaml:"name"`
	FileCount int    `json:"file_count" yaml:"file_count"`
}

type CaptureResult struct {
	Languages []LanguageSummary     `json:"languages" yaml:"languages"`
	Groups    []DirGroup            `json:"groups" yaml:"groups"`
	Snapshots []worker.FileSnapshot `json:"-" yaml:"-"`
}

func languageSummaries(groups map[string]*scanner.LangGroup) []LanguageSummary {
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}

	sort.Strings(names)

	out := make([]LanguageSummary, 0, len(names))
	for _, name := range names {
		group := groups[name]
		out = append(out, LanguageSummary{
			Name:      name,
			FileCount: len(group.Paths),
		})
	}

	return out
}

func Capture(opts CaptureOptions) (*CaptureResult, error) {
	if opts.RulesDir == "" {
		return nil, fmt.Errorf("capture rules dir is empty: %s", opts.RulesDir)
	}

	cfg, err := config.Load(opts.RulesDir)
	if err != nil {
		cfg = config.Default()
	}

	// Compiled once, shared read-only across all workers — safe.
	routeExtractor := routes.NewExtractor(cfg)

	// Phase 1: walk the directory — no file content read yet.
	groups, err := scanner.GroupByLanguage(opts.TargetDir)
	if err != nil {
		return nil, fmt.Errorf("grouping files by language: %w", err)
	}

	// Phase 2: one worker goroutine per language group.
	var (
		drainWg   sync.WaitGroup
		mu        sync.Mutex
		snapshots []worker.FileSnapshot
	)

	for _, group := range groups {
		drainWg.Add(1)

		w := worker.New(group, opts.RulesDir, routeExtractor)

		// Drain goroutine: consumes results as they arrive.
		go func(w *worker.Worker) {
			var wg sync.WaitGroup
			wg.Add(1)
			w.Run(&wg)
			wg.Wait()
		}(w)

		go func(w *worker.Worker) {
			defer drainWg.Done()

			for snap := range w.Results {
				mu.Lock()
				snapshots = append(snapshots, snap)
				mu.Unlock()
			}
		}(w)
	}

	drainWg.Wait()

	grouped := groupSnapshotsByDir(opts.TargetDir, snapshots)

	return &CaptureResult{
		Languages: languageSummaries(groups),
		Groups:    grouped,
		Snapshots: snapshots,
	}, nil
}

func groupSnapshotsByDir(targetDir string, snapshots []worker.FileSnapshot) []DirGroup {
	byDir := make(map[string][]worker.FileSnapshot)

	for _, snap := range snapshots {
		rel, _ := filepath.Rel(targetDir, filepath.Dir(snap.Path))
		byDir[rel] = append(byDir[rel], snap)
	}

	dirs := make([]string, 0, len(byDir))
	for dir := range byDir {
		dirs = append(dirs, dir)
	}

	sort.Strings(dirs)

	grouped := make([]DirGroup, 0, len(dirs))
	for _, dir := range dirs {
		grouped = append(grouped, DirGroup{
			Dir:   dir,
			Files: byDir[dir],
		})
	}

	return grouped
}
