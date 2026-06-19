package core

// ToolDefinition represents a single tool from commands.json
type ToolDefinition struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Namespace       string        `json:"namespace"`
	Description     string        `json:"description"`
	Author          interface{}   `json:"author,omitempty"`
	Version         string        `json:"version"`
	Capabilities    []string      `json:"capabilities,omitempty"`
	Platforms       []string      `json:"platforms,omitempty"`
	Architectures   []string      `json:"architectures,omitempty"`
	RiskLevel       string        `json:"risk_level,omitempty"`
	ExecutionPolicy string        `json:"execution_policy,omitempty"`
	TrustLevel      string        `json:"trust_level,omitempty"`
	Features        []string      `json:"features,omitempty"`
	Techniques      []string      `json:"techniques,omitempty"`
	Parameters      []Parameter   `json:"parameters,omitempty"`
	Execution       Execution     `json:"execution"`
	Install         []Install     `json:"install,omitempty"`
	Phase           string        `json:"phase,omitempty"`
	MitreIDs        []string      `json:"mitre_ids,omitempty"`
	Dependencies    []string      `json:"dependencies,omitempty"`
	RelatedTools    []string      `json:"related_tools,omitempty"`
	GlobalVars      map[string]string `json:"global_vars,omitempty"`
}

type Parameter struct {
	Name            string      `json:"name"`
	TemplateKey     string      `json:"template_key,omitempty"`
	Type            string      `json:"type"`
	Required        bool        `json:"required"`
	DefaultValue    interface{} `json:"default_value,omitempty"`
	Description     string      `json:"description"`
	Flag            string      `json:"flag,omitempty"`
	Format          string      `json:"format,omitempty"`
	PositionalOrder int         `json:"positional_order,omitempty"`
	Aliases         []string    `json:"aliases,omitempty"`
	Enum         []string    `json:"enum,omitempty"`
	Pattern      string      `json:"pattern,omitempty"`
	Minimum      *float64    `json:"minimum,omitempty"`
	Maximum      *float64    `json:"maximum,omitempty"`
}

type Execution struct {
	Template       string            `json:"template"`
	Sandbox        string            `json:"sandbox"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
	Shell          bool              `json:"shell,omitempty"`
	Workdir        string            `json:"workdir,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Container      *Container        `json:"container,omitempty"`
}

type Container struct {
	Image string `json:"image"`
}

type Install struct {
	Method      string   `json:"method"`
	PackageName string   `json:"package_name,omitempty"`
	RepoURL     string   `json:"repo_url,omitempty"`
	Commands    []string `json:"commands"`
}

// InstallStatus tracks whether a tool is installed and how
type InstallStatus struct {
	ToolName       string
	OnPath         bool
	PathLocation   string
	PackageManager string
	Version        string
	DockerImage    bool
}

// ToolSource represents from where we load tool data
type ToolSource string

const (
	SourceCache ToolSource = "cache"
	SourceSite  ToolSource = "site"
	SourceFile  ToolSource = "file"
)

// QueryFilter holds parsed structured query fields
type QueryFilter struct {
	Domains    []string          // domain filter values
	Actions    []string          // action filter values
	Tools      []string          // tool filter values
	Flags      []string          // flag filter values
	Phase      string            // phase filter
	Services   []string          // service filter
	Techniques []string          // technique filter
	MitreIDs   []string          // MITRE ID filter
	Risk       string            // risk level filter
	Platforms  []string          // platform filter
	Protocols  []string          // protocol filter
	Port       string            // port parameter value
	Target     string            // target parameter value
	Username   string            // username parameter value
	Password   string            // password parameter value
	Hash       string            // hash parameter value
	Wordlist   string            // wordlist parameter value
	Output     string            // output format parameter value
	RawFlags   string            // raw flags string (unparsed)
	Params     map[string]string // structured parameter map (key=template_key, value=raw value)
	Source     string            // query source type: "structured" or "free-text"
}

// QueryResult represents a single tool match from a query
type QueryResult struct {
	ToolID      string  `json:"tool_id"`
	ToolName    string  `json:"tool_name"`
	Namespace   string  `json:"namespace"`
	Domain      string  `json:"domain"`
	Command     string  `json:"command"`
	Confidence  float64 `json:"confidence"`
	Phase       string  `json:"phase"`
	RiskLevel   string  `json:"risk_level,omitempty"`
	Description string  `json:"description"`
	Explanation string  `json:"explanation"`
}

// QueriesIndex mirrors the queries.json structure from the AutoXMate site
type QueriesIndex struct {
	Actions         map[string]ActionIndexEntry    `json:"actions"`
	Domains         map[string][]string           `json:"domains"`
	Tools           map[string]QueryToolDef       `json:"tools"`
	Presets         []QueryPreset                 `json:"presets"`
	Keywords        map[string][]string           `json:"keywords"`
	CuratedActions  []string                      `json:"curated_actions"`
}

// ActionIndexEntry is an entry in the actions index
type ActionIndexEntry struct {
	DefaultFlags       string                 `json:"default_flags"`
	DefaultTargetParam string                 `json:"default_target_param"`
	ToolDefaults       map[string]ToolDefault `json:"tool_defaults"`
	Tools              []ActionToolRef        `json:"tools"`
}

// ActionToolRef is a tool reference within an action index
type ActionToolRef struct {
	ToolID      string `json:"tool_id"`
	ToolName    string `json:"tool_name"`
	Capability  string `json:"capability"`
	Confidence  string `json:"confidence"` // "exact", "partial", or "inferred"
}

// QueryToolDef is a minimal tool definition for query purposes
type QueryToolDef struct {
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	Namespace         string             `json:"namespace"`
	Domain            string             `json:"domain"`
	Description       string             `json:"description"`
	Capabilities      []string           `json:"capabilities"`
	Phase             string             `json:"phase"`
	Techniques        []string           `json:"techniques"`
	RiskLevel         string             `json:"risk_level"`
	Services          []string           `json:"services"`
	Platforms         []string           `json:"platforms"`
	MitreIDs          []string           `json:"mitre_ids"`
	ExecutionTemplate string             `json:"execution_template"`
	Parameters        []QueryParamDef    `json:"parameters"`
}

// QueryParamDef is a minimal parameter definition for query matching
type QueryParamDef struct {
	Name            string      `json:"name"`
	TemplateKey     string      `json:"template_key"`
	Type            string      `json:"type"`
	Required        bool        `json:"required"`
	DefaultVal      interface{} `json:"default_value"`
	Flag            string      `json:"flag,omitempty"`
	Format          string      `json:"format,omitempty"`
	PositionalOrder int         `json:"positional_order,omitempty"`
	Aliases         []string    `json:"aliases"`
	Enum            []string    `json:"enum"`
}

// QueryPreset is a pre-built example query-command pair
type QueryPreset struct {
	Query       string `json:"query"`
	Command     string `json:"command"`
	Description string `json:"description"`
	ToolID      string `json:"tool_id"`
	ToolName    string `json:"tool_name"`
}

// ActionTaxonomyEntry is the structure of an entry in action-taxonomy.json
type ActionTaxonomyEntry struct {
	Description      string                `json:"description"`
	Capabilities     []string              `json:"capabilities"`
	DefaultFlags     string                `json:"default_flags"`
	DefaultTargetParam string              `json:"default_target_param"`
	Keywords         []string              `json:"keywords"`
	ToolDefaults     map[string]ToolDefault `json:"tool_defaults"`
}

// ToolDefault specifies per-tool overrides for an action
type ToolDefault struct {
	Flags            string `json:"flags"`
	TargetParam      string `json:"target_param"`
	TemplateOverride string `json:"template_override,omitempty"`
}
