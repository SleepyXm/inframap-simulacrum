package simulation

import (
	"bytes"
	"db-seeder/simulation/corpus"
	"db-seeder/simulation/types"
	"db-seeder/walker"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Stage1Config is everything RunStage1 needs to know:
// where the app is running, what endpoints exist, and how to extract the auth token.
type Stage1Config struct {
	BaseURL    string                 // e.g. "http://localhost:8080"
	Context    *walker.ProjectContext // walker output — the list of known endpoints
	AuthConfig types.AuthConfig       // tells us where to find the token in the login response
}

// findAuthEndpoints scans the walker output for POST /signup and POST /login.
// We need these two paths to run stage 1 — if either is missing we can't proceed.
func findAuthEndpoints(ctx *walker.ProjectContext) (signup, login string) {
	// ctx.Files is every source file the walker scanned
	for _, file := range ctx.Files {
		// each file has a list of endpoints the walker found in it
		for _, ep := range file.Endpoints {
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

// RunStage1 takes every virtual user through signup then login.
// Users that succeed get written to the corpus — users that fail are skipped and logged.
// The corpus is the verified list of users that actually exist in the system.
func RunStage1(cfg Stage1Config, users []types.VirtualUser, c *corpus.Corpus) error {
	signup, login := findAuthEndpoints(cfg.Context)
	if signup == "" || login == "" {
		return fmt.Errorf("could not find signup/login endpoints in walker output")
	}

	for _, u := range users {
		entry, err := runUserStage1(cfg, signup, login, u)
		if err != nil {
			// user failed signup or login — skip them, don't add to corpus
			fmt.Printf("user %s failed stage1: %v\n", u.ID, err)
			continue
		}
		if err := c.Add(entry); err != nil {
			fmt.Printf("user %s: failed to write corpus: %v\n", u.ID, err)
		}
	}
	return nil
}

// runUserStage1 runs a single virtual user through signup then login.
// Returns a corpus entry containing the user's seed data, session token, and request metrics.
func runUserStage1(cfg Stage1Config, signupPath, loginPath string, u types.VirtualUser) (corpus.Entry, error) {
	// start building the corpus entry — we append to it as the user progresses
	entry := corpus.Entry{
		ID:      u.ID,
		Seed:    u.Seed,                  // the credentials/profile used to register
		Session: map[string]string{},     // filled in after login
		Metrics: []types.RequestMetric{}, // one metric per request made
	}

	// marshal the user's seed data (email, password, etc.) into JSON for the request body
	body, _ := json.Marshal(u.Seed)

	// --- signup ---
	metric, _, _, err := doRequest(http.MethodPost, cfg.BaseURL+signupPath, body, nil)
	entry.Metrics = append(entry.Metrics, metric) // record the signup attempt regardless of outcome
	if err != nil {
		return entry, fmt.Errorf("signup request failed: %w", err)
	}
	if metric.StatusCode != http.StatusOK && metric.StatusCode != http.StatusCreated {
		return entry, fmt.Errorf("signup returned %d", metric.StatusCode)
	}

	// --- login ---
	// respBody contains the login response — we need it to extract the session token
	metric, respBody, respHeaders, err := doRequest(http.MethodPost, cfg.BaseURL+loginPath, body, nil)
	entry.Metrics = append(entry.Metrics, metric)
	if err != nil {
		return entry, fmt.Errorf("login request failed: %w", err)
	}
	if metric.StatusCode != http.StatusOK {
		return entry, fmt.Errorf("login returned %d", metric.StatusCode)
	}

	// DEBUG — remove once you know the key
	fmt.Printf("login response body: %s\n", string(respBody))

	// pull the session token out of the login response using whatever AuthConfig says
	session, err := extractSession(respBody, respHeaders, cfg.AuthConfig)
	if err != nil {
		return entry, fmt.Errorf("extracting session: %w", err)
	}
	entry.Session = session

	return entry, nil
}

// doRequest fires an HTTP request and returns a metric capturing what happened + the response body.
// headers is optional — pass nil if you don't need any extra headers.
func doRequest(method, url string, body []byte, headers map[string]string) (types.RequestMetric, []byte, http.Header, error) {
	// start building the metric — we'll fill in status and latency after the request
	metric := types.RequestMetric{
		Endpoint:  url,
		Method:    method,
		Timestamp: time.Now(),
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return metric, nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, val := range headers {
		req.Header.Set(key, val)
	}

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	metric.Latency = time.Since(start) // captured regardless of whether the request succeeded
	if err != nil {
		return metric, nil, nil, err
	}
	defer resp.Body.Close()

	metric.StatusCode = resp.StatusCode

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return metric, nil, nil, err
	}

	return metric, respBody, resp.Header, nil
}

// extractSession pulls the auth token out of the login response and returns it as a session map.
// How it does that depends on AuthConfig — the token might be in the JSON body or a response header.
func extractSession(body []byte, headers http.Header, cfg types.AuthConfig) (map[string]string, error) {
	session := map[string]string{}

	switch cfg.TokenSource {
	case types.TokenFromBody:
		// parse the response body as JSON and look for the token key
		var parsed map[string]interface{}
		if err := json.Unmarshal(body, &parsed); err != nil {
			return session, fmt.Errorf("parsing login response body: %w", err)
		}
		val, ok := parsed[cfg.TokenKey] // cfg.TokenKey is something like "token" or "access_token"
		if !ok {
			return session, fmt.Errorf("token key %q not found in response", cfg.TokenKey)
		}
		token := fmt.Sprintf("%v", val)
		if cfg.Prefix != "" {
			token = cfg.Prefix + token // e.g. prepend "Bearer " to the token value
		}
		session[cfg.HeaderName] = token // e.g. session["Authorization"] = "Bearer abc123"

	case types.TokenFromHeader:
		raw := headers.Get(cfg.TokenKey) // e.g. "Set-Cookie"
		if raw == "" {
			return session, fmt.Errorf("header %q not found in response", cfg.TokenKey)
		}
		// store the raw cookie string — sent back as "Cookie: <value>" on subsequent requests
		session[cfg.HeaderName] = raw
	}

	return session, nil
}
