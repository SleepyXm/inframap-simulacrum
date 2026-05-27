package simulation

import (
	"db-seeder/handlers"
	"db-seeder/simulation/corpus"
	"db-seeder/simulation/types"
	"db-seeder/structs"
	"db-seeder/tools"
	"db-seeder/walker"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

type SimTool struct {
	ctx   *walker.ProjectContext
	dir   string
	ready bool
}

func NewTool(dir string) (*SimTool, error) {
	data, err := os.ReadFile(filepath.Join(dir, ".walker/context.json"))
	if err != nil {
		return nil, fmt.Errorf("sim: no walker output found — run walker first: %w", err)
	}
	var ctx walker.ProjectContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("sim: invalid walker output: %w", err)
	}
	if len(ctx.Files) == 0 {
		return nil, fmt.Errorf("sim: walker context is empty — run walker first")
	}
	return &SimTool{ctx: &ctx, dir: dir, ready: true}, nil
}

func (t *SimTool) Name() string    { return "Simulation" }
func (t *SimTool) Available() bool { return t.ready }
func (t *SimTool) Prompt() string  { return "http://localhost:9000/api/auth" } // base URL, not a path

func (t *SimTool) Run(input string) tea.Cmd {
	return func() tea.Msg {
		result := t.run(input)
		return tools.ToolDoneMsg{Tool: t.Name(), Result: result}
	}
}

func (t *SimTool) run(baseURL string) tools.ToolResult {
	c := corpus.New(filepath.Join(t.dir, corpus.DefaultPath))

	users := generateVirtualUsers(50) // sensible default, make configurable later

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
	// failed users never made it into corpus — tracked separately in future
	return 0
}

// GenPerson wraps the existing generator — keeps tool.go self-contained.
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
