package simulation

import (
	"db-seeder/handlers"
	"db-seeder/simulation/corpus"
	"db-seeder/simulation/types"
	"db-seeder/structs"
	"db-seeder/tools"
	walker_types "db-seeder/walker/types"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type SimTool struct {
	ctx *walker_types.ProjectContext
	dir string
}

func NewTool(dir string) (*SimTool, error) {
	if strings.TrimSpace(dir) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("sim: could not determine working directory: %w", err)
		}
		dir = cwd
	}

	return &SimTool{dir: dir}, nil
}

func (t *SimTool) Name() string { return "Simulation" }

func (t *SimTool) Available() bool {
	return t.loadContext() == nil
}

func (t *SimTool) Prompt() string { return "http://localhost:9000/api/auth" } // base URL, not a path

func (t *SimTool) contextPath() string {
	return filepath.Join(t.dir, ".walker/context.json")
}

func (t *SimTool) loadContext() error {
	data, err := os.ReadFile(t.contextPath())
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("sim: no walker context found - run walker first: %w", err)
		}
		return fmt.Errorf("sim: reading walker context: %w", err)
	}

	var ctx walker_types.ProjectContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		return fmt.Errorf("sim: invalid walker context: %w", err)
	}
	if len(ctx.Files) == 0 {
		return fmt.Errorf("sim: walker context is empty - run walker first")
	}

	t.ctx = &ctx
	return nil
}

func (t *SimTool) Run(input string) tea.Cmd {
	return func() tea.Msg {
		result := t.run(input)
		return tools.ToolDoneMsg{Tool: t.Name(), Result: result}
	}
}

func (t *SimTool) run(baseURL string) tools.ToolResult {
	if err := t.loadContext(); err != nil {
		return tools.ToolResult{Err: err}
	}

	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = t.Prompt()
	}

	c := corpus.New(filepath.Join(t.dir, corpus.DefaultPath))

	users := generateVirtualUsers(1000) // sensible default, make configurable later

	cfg := Stage1Config{
		BaseURL: baseURL,
		Context: t.ctx,
		AuthConfig: types.AuthConfig{
			TokenSource: types.TokenFromHeader,
			TokenKey:    "Set-Cookie", // access_token CHANGE FOR YOUR TYPE. Realistically needs changes because of how dynamic tokens will be, but this is a start.
			HeaderName:  "Cookie",
			Prefix:      "", //"Bearer "
		},
		//AuthConfig: types.AuthConfig{
		//	TokenSource: types.TokenFromBody,
		//	TokenKey:    "access_token", // access_token CHANGE FOR YOUR TYPE. Realistically needs changes because of how dynamic tokens will be, but this is a start.
		//	HeaderName:  "Authorization",
		//	Prefix:      "", //"Bearer "
		//},
	}

	if err := RunStage1(cfg, users, c); err != nil {
		return tools.ToolResult{Err: err}
	}

	return tools.ToolResult{
		Summary: summariseToLines(c),
		Outputs: []string{filepath.Join(t.dir, corpus.DefaultPath)},
	}
}

func generateVirtualUsers(n int) []types.VirtualUser {
	users := make([]types.VirtualUser, 0, n)
	for i := 0; i < n; i++ {
		p := GenPerson()
		users = append(users, types.VirtualUser{
			ID: fmt.Sprintf("vu-%d", i),
			Seed: map[string]string{
				"first_name": p.Firstname,
				"last_name":  p.Lastname,
				"email":      p.Email,
				"username":   p.Username,
				"password":   p.Password,
			},
			State: types.VUState{
				Session: map[string]string{},
			},
		})
	}
	return users
}

func summariseToLines(c *corpus.Corpus) []tools.SummaryLine {
	entries := c.Entries()
	total := len(entries)

	totalRequests := 0
	totalLatency := int64(0)
	for _, e := range entries {
		totalRequests += len(e.Metrics)
		for _, m := range e.Metrics {
			totalLatency += m.Latency.Milliseconds()
		}
	}

	avgLatency := int64(0)
	if totalRequests > 0 {
		avgLatency = totalLatency / int64(totalRequests)
	}

	return []tools.SummaryLine{
		{Label: "users attempted", Value: fmt.Sprintf("%d", total+countFailed(c))},
		{Label: "users in corpus", Value: fmt.Sprintf("%d", total)},
		{Label: "total requests", Value: fmt.Sprintf("%d", totalRequests)},
		{Label: "avg latency", Value: fmt.Sprintf("%dms", avgLatency)},
	}
}

func countFailed(c *corpus.Corpus) int {
	// Failed users never made it into corpus; track them separately later.
	return 0
}

// GenPerson wraps the existing generator and keeps tool.go self-contained.
func GenPerson() structs.Person {
	firstname := structs.FirstNames[rand.Intn(len(structs.FirstNames))]
	lastname := structs.LastNames[rand.Intn(len(structs.LastNames))]
	return structs.Person{
		Firstname: firstname,
		Lastname:  lastname,
		Email:     handlers.GenEmail(firstname, lastname),
		Username:  handlers.GenUsername(firstname, lastname),
		Password:  handlers.GenPassword(""),
	}
}
