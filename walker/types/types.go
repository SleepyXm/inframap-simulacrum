package types

import (
	"regexp"

	"db-seeder/walker/core/syntax"

	sitter "github.com/smacker/go-tree-sitter"
)

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
	Path    string         `yaml:"path" json:"path"`
	Structs interface{}    `yaml:"structs"` // "auto" or []string
	Output  OutputConfig   `yaml:"output" json:"output"`
	Capture CaptureConfig  `yaml:"capture" json:"capture"`
	Scanner ScannerConfig  `yaml:"scanner" json:"scanner"`
	Bracket BracketConfig  `yaml:"bracket_tracker"`
	Custom  CustomPatterns `yaml:"custom_patterns"`
	Context ContextConfig  `yaml:"context"`
}

type CaptureConfig struct {
	ConfigPath string `yaml:"config_path" json:"config_path"`
	RulesDir   string `yaml:"rules_dir" json:"rules_dir"`
}

type OutputConfig struct {
	JSON JSONOutputConfig `yaml:"json"`
	YAML YAMLOutputConfig `yaml:"yaml"`

	SkipDirs    []string `json:"skip_dirs" yaml:"skip_dirs"`
	SkipFiles   []string `json:"skip_files" yaml:"skip_files"`
	IncludeOnly []string `json:"include_only" yaml:"include_only"`
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
// ──────────────────────────────────────────────────────────────────────────────
// File
// ──────────────────────────────────────────────────────────────────────────────

type SourceFile struct {
	Path     string
	Content  []byte
	Ext      string
	Language *sitter.Language
	Syntax   syntax.LangSyntax
}

// ──────────────────────────────────────────────────────────────────────────────
// Primitive / shared
// ──────────────────────────────────────────────────────────────────────────────

type Primitive struct {
	Kind        string
	File        string
	StartLine   int
	StartColumn int
	EndLine     int
	EndColumn   int
	Data        map[string]string
}

// ──────────────────────────────────────────────────────────────────────────────
// Routes
// ──────────────────────────────────────────────────────────────────────────────

type RuleDef struct {
	Name     string `yaml:"name"`
	Pattern  string `yaml:"pattern"`
	Language string `yaml:"language"`
	Multi    bool   `yaml:"multi"`
}

type PrefixRuleDef struct {
	Name    string `yaml:"name"`
	Pattern string `yaml:"pattern"`
}

type RoutePattern struct {
	Re          *regexp.Regexp
	Name        string
	MethodIdx   int
	PathIdx     int
	ReceiverIdx int
	Language    string
	Multi       bool
}

type PrefixRule struct {
	Re          *regexp.Regexp
	VarIdx      int
	ReceiverIdx int
	PrefixIdx   int
}

// ──────────────────────────────────────────────────────────────────────────────
// Imports
// ──────────────────────────────────────────────────────────────────────────────

type ImportRuleDef struct {
	Name     string `yaml:"name"`
	Pattern  string `yaml:"pattern"`
	Language string `yaml:"language"`
}

type ImportRule struct {
	Re         *regexp.Regexp
	Language   string
	PathIdx    int // "import" group — the module/package path
	NamesIdx   int // "names" group — named imports: { A, B } or "from X import A, B"
	AliasIdx   int // "alias" group — "import x as y" or "import alias path"
	ModuleIdx  int // "module" group — Python "from <module> import ..."
	ImportsIdx int // "imports" group — Python bare "import os, sys"
}

type UsageSite struct {
	Line     int
	Function string
}

type Import struct {
	Path   string
	Alias  string
	Names  []string
	Usages map[string][]UsageSite
}

// ──────────────────────────────────────────────────────────────────────────────
// Functions & parameters
// ──────────────────────────────────────────────────────────────────────────────

type FunctionRuleDef struct {
	Name     string `yaml:"name"`
	Pattern  string `yaml:"pattern"`
	Language string `yaml:"language"`
}

type FunctionRule struct {
	Re       *regexp.Regexp
	NameIdx  int
	Language string
}

type ParameterRuleDef struct {
	Name     string `yaml:"name"`
	Pattern  string `yaml:"pattern"`
	Language string `yaml:"language"`
}

type ParameterRule struct {
	Re       *regexp.Regexp
	Language string
}

type Param struct {
	Name string
	Type string
	Raw  string
}

type FunctionDef struct {
	Name        string
	StartLine   int
	EndLine     int
	Params      []Param
	RawParams   string
	Returns     []ReturnDef     `json:"returns,omitempty"`
	Assignments []AssignmentDef `json:"assignments,omitempty"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Classes & fields
// ──────────────────────────────────────────────────────────────────────────────

type ClassRuleDef struct {
	Name     string `yaml:"name"`
	Pattern  string `yaml:"pattern"`
	Language string `yaml:"language"`
}

type ClassRule struct {
	Re       *regexp.Regexp
	NameIdx  int
	BasesIdx int
	Language string
}

type FieldRuleDef struct {
	Name     string `yaml:"name"`
	Pattern  string `yaml:"pattern"`
	Language string `yaml:"language"`
}

type FieldRule struct {
	Re         *regexp.Regexp
	NameIdx    int
	TypeIdx    int
	TagIdx     int
	DefaultIdx int
	Language   string
}

type ClassDef struct {
	Name      string            `json:"name"`
	Bases     []string          `json:"bases,omitempty"`
	StartLine int               `json:"startLine"`
	EndLine   int               `json:"endLine"`
	Fields    []FieldDef        `json:"fields,omitempty"`
	Data      map[string]string `json:"data,omitempty"`
}

type FieldDef struct {
	Name    string `json:"name"`
	Type    string `json:"type,omitempty"`
	Tag     string `json:"tag,omitempty"`
	Default string `json:"default,omitempty"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Control flow — loops, assignments, returns
// ──────────────────────────────────────────────────────────────────────────────

type LoopRuleDef struct {
	Name     string `yaml:"name"`
	Pattern  string `yaml:"pattern"`
	Language string `yaml:"language"`
}

type LoopRule struct {
	Re       *regexp.Regexp
	Language string
}

type AssignmentDef struct {
	Var      string `json:"var"`
	Value    string `json:"value,omitempty"`
	Line     int    `json:"line"`
	Function string `json:"function,omitempty"`
}

type AssignmentRuleDef struct {
	Language string `yaml:"language"`
	Name     string `yaml:"name"`
	Pattern  string `yaml:"pattern"`
	Var      string
	Value    string
	Line     int
	Function string
}

//type AssignmentRuleDef struct {
//	Name     string `yaml:"name"`
//	Pattern  string `yaml:"pattern"`
//	Language string `yaml:"language"`
//}

type AssignmentRule struct {
	Re       *regexp.Regexp
	VarIdx   int
	ValueIdx int
	Language string
}

//type AssignmentRule struct {
//	Re     *regexp.Regexp
//	VarIdx int
//}

type ReturnRuleDef struct {
	Name     string `yaml:"name"`
	Pattern  string `yaml:"pattern"`
	Language string `yaml:"language"`
}

type ReturnRule struct {
	Re       *regexp.Regexp
	Name     string
	ValueIdx int
	Language string
}

type ReturnDef struct {
	Mechanism string `json:"mechanism"`
	Shape     string `json:"shape"`
	Value     string `json:"value,omitempty"`
	Line      int    `json:"line"`
	Function  string `json:"function,omitempty"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Config — merged in-memory representation of all rule files
// ──────────────────────────────────────────────────────────────────────────────

// Config is the merged result of loading all rule YAML files.
// Use LoadConfig to populate it from a directory containing the split files.
type Config struct {
	Name string `yaml:"name"`
	// routes.yml
	RouteMethods []string        `yaml:"route_methods"`
	RouteRules   []RuleDef       `yaml:"route_rules"`
	PrefixRules  []PrefixRuleDef `yaml:"prefix_rules"`
	// imports.yml
	ImportRules []ImportRuleDef `yaml:"import_rules"`
	// functions.yml
	FunctionRules  []FunctionRuleDef  `yaml:"function_rules"`
	ParameterRules []ParameterRuleDef `yaml:"parameter_rules"`
	// types.yml
	ClassRules []ClassRuleDef `yaml:"class_rules"`
	FieldRules []FieldRuleDef `yaml:"field_rules"`
	// control_flow.yml
	LoopRules        []LoopRuleDef        `yaml:"loop_rules"`
	AssignmentRules  []AssignmentRuleDef  `yaml:"assignment_rules"`
	ConditionalRules []ConditionalRuleDef `yaml:"conditional_rules"`
	ReturnRules      []ReturnRuleDef      `yaml:"return_rules"`
}

// Per-file structs used by LoadConfig — each maps to exactly one YAML file.

type RoutesFile struct {
	Name         string          `yaml:"name"`
	RouteMethods []string        `yaml:"route_methods"`
	PrefixRules  []PrefixRuleDef `yaml:"prefix_rules"`
	RouteRules   []RuleDef       `yaml:"route_rules"`
}

type ImportsFile struct {
	ImportRules []ImportRuleDef `yaml:"import_rules"`
}

type FunctionsFile struct {
	FunctionRules  []FunctionRuleDef  `yaml:"function_rules"`
	ParameterRules []ParameterRuleDef `yaml:"parameter_rules"`
}

type TypesFile struct {
	ClassRules []ClassRuleDef `yaml:"class_rules"`
	FieldRules []FieldRuleDef `yaml:"field_rules"`
}

type ControlFlowFile struct {
	LoopRules       []LoopRuleDef       `yaml:"loop_rules"`
	AssignmentRules []AssignmentRuleDef `yaml:"assignment_rules"`
	ReturnRules     []ReturnRuleDef     `yaml:"return_rules"`
}

// Conditional Types ----------------- //

type ConditionalRuleDef struct {
	Name     string `yaml:"name"`
	Pattern  string `yaml:"pattern"`
	Language string `yaml:"language"`
	Kind     string `yaml:"kind"`
}

type ConditionalRule struct {
	Re           *regexp.Regexp
	Kind         string
	Language     string
	ConditionIdx int
}

type ConditionalDef struct {
	Kind      string `json:"kind"`
	Condition string `json:"condition,omitempty"`
	Line      int    `json:"line"`
	Function  string `json:"function,omitempty"`
}

type ConditionalsFile struct {
	ConditionalRules []ConditionalRuleDef `yaml:"conditional_rules"`
}
