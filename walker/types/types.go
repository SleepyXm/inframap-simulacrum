package types

// Language is a detected source language.
type Language string

const (
	LangGo      Language = "go"
	LangPython  Language = "python"
	LangUnknown Language = "unknown"
)

// ExtensionLanguage maps file extensions to Language constants.
var ExtensionLanguage = map[string]Language{
	".go": LangGo,
	".py": LangPython,
	".js": LangUnknown,
	".ts": LangUnknown,
}

// ---------------------------------------------------------------------------
// Struct file types (loaded from walkers/patterns/*.yml)
// ---------------------------------------------------------------------------

// Pattern is a single named regex pattern from a struct file.
type Pattern struct {
	Name        string `yaml:"name"`
	Pattern     string `yaml:"pattern"`
	Method      string `yaml:"method,omitempty"`
	MethodGroup int    `yaml:"method_group,omitempty"`
	PathGroup   int    `yaml:"path_group,omitempty"`
	Library     string `yaml:"library,omitempty"`
	Kind        string `yaml:"kind,omitempty"`
	Note        string `yaml:"note,omitempty"`
}

// SkipRules defines what to skip during scanning for a given language.
type SkipRules struct {
	Dirs  []string `yaml:"dirs"`
	Files []string `yaml:"files"`
}

// LanguageStruct is the parsed representation of a walker/patterns/*.yml file.
type LanguageStruct struct {
	Language           Language  `yaml:"language"`
	Extensions         []string  `yaml:"extensions"`
	RouterRegistration []Pattern `yaml:"router_registration"`
	GroupPrefix        []Pattern `yaml:"group_prefix"`
	DBCalls            []Pattern `yaml:"db_calls"`
	Models             []Pattern `yaml:"models"`
	Skip               SkipRules `yaml:"skip"`
}

// ---------------------------------------------------------------------------
// Walkerfile types (loaded from walkerfile.yml)
// ---------------------------------------------------------------------------

// WalkerFile is the parsed representation of walkerfile.yml.
type WalkerFile struct {
	Path    string         `yaml:"path"`
	Structs interface{}    `yaml:"structs"` // "auto" or []string
	Output  OutputConfig   `yaml:"output"`
	Scanner ScannerConfig  `yaml:"scanner"`
	Bracket BracketConfig  `yaml:"bracket_tracker"`
	Custom  CustomPatterns `yaml:"custom_patterns"`
	Context ContextConfig  `yaml:"context"`
}

type OutputConfig struct {
	JSON JSONOutputConfig `yaml:"json"`
	YAML YAMLOutputConfig `yaml:"yaml"`
}

type JSONOutputConfig struct {
	Path   string `yaml:"path"`
	Pretty bool   `yaml:"pretty"`
}

type YAMLOutputConfig struct {
	Path string `yaml:"path"`
}

type ScannerConfig struct {
	FollowSymlinks bool     `yaml:"follow_symlinks"`
	MaxDepth       int      `yaml:"max_depth"`
	SkipDirs       []string `yaml:"skip_dirs"`
	SkipFiles      []string `yaml:"skip_files"`
	IncludeOnly    []string `yaml:"include_only"`
}

type BracketConfig struct {
	MaxDepth           int  `yaml:"max_depth"`
	CrossFunctionScope bool `yaml:"cross_function_scope"`
}

type CustomPatterns struct {
	RouterRegistration []Pattern `yaml:"router_registration"`
	GroupPrefix        []Pattern `yaml:"group_prefix"`
	DBCalls            []Pattern `yaml:"db_calls"`
	Models             []Pattern `yaml:"models"`
}

type ContextConfig struct {
	IncludeRawLine     bool `yaml:"include_raw_line"`
	IncludeLineNumbers bool `yaml:"include_line_numbers"`
	IncludeHandler     bool `yaml:"include_handler"`
	IncludeDBKind      bool `yaml:"include_db_kind"`
	GroupByFile        bool `yaml:"group_by_file"`
}

// ---------------------------------------------------------------------------
// Cross-file prefix resolution
// ---------------------------------------------------------------------------

// RouteRegistration records a call-site where a sub-router is handed to a
// function, e.g.:
//
//	routes.RegisterAuthRoutes(api.Group("/auth"), db)
//
// FuncName is "RegisterAuthRoutes"; ResolvedPrefix is "/api/auth" (already
// flattened by the first-pass scan of the entry-point file).
// The runner uses this to find the file that defines FuncName and re-scans
// it with ResolvedPrefix as an inherited prefix.
type RouteRegistration struct {
	FuncName       string // name of the registration function called
	ResolvedPrefix string // fully resolved prefix at the call site
	Line           int    `json:"line,omitempty"`
}

// ---------------------------------------------------------------------------
// Output types (written to context.json and context.yml)
// ---------------------------------------------------------------------------

// ProjectContext is the root output type — written to context.json.
type ProjectContext struct {
	Files []FileContext `json:"files"`
}

// FileContext holds all extracted data for a single source file.
type FileContext struct {
	Path      string     `json:"path"`
	Language  Language   `json:"language"`
	Endpoints []Endpoint `json:"endpoints,omitempty"`
	DBCalls   []DBCall   `json:"db_calls,omitempty"`
	Models    []ModelDef `json:"models,omitempty"`
}

// ModelDef represents a detected data model (SQLAlchemy, Pydantic, Go struct).
type ModelDef struct {
	Name   string            `json:"name"`
	Kind   string            `json:"kind"` // "db_model", "request_model", "struct"
	Line   int               `json:"line,omitempty"`
	Fields map[string]string `json:"fields,omitempty"`
}

// Endpoint represents a single matched route registration.
type Endpoint struct {
	Method   string `json:"method"`
	Path     string `json:"path"`
	FullPath string `json:"full_path,omitempty"` // prefix-resolved
	Handler  string `json:"handler,omitempty"`
	Line     int    `json:"line,omitempty"`
	RawLine  string `json:"raw_line,omitempty"`
}

// DBCall represents a single matched database call site.
type DBCall struct {
	Library string `json:"library,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Query   string `json:"query,omitempty"` // extracted SQL if available
	Handler string `json:"handler,omitempty"`
	Line    int    `json:"line,omitempty"`
	RawLine string `json:"raw_line,omitempty"`
}

/// ------------------ Internal types (not serialized) ------------------
