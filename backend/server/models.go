package server

type Endpoint struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Method      string   `json:"method"`
	PathPattern string   `json:"pathPattern"`
	Enabled     bool     `json:"enabled"`
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
	CreatedAt   int64    `json:"createdAt"`
	UpdatedAt   int64    `json:"updatedAt"`
}

type Rule struct {
	ID         string         `json:"id"`
	EndpointID string         `json:"endpointId"`
	Name       string         `json:"name"`
	Enabled    bool           `json:"enabled"`
	Priority   int            `json:"priority"`
	Weight     int            `json:"weight"`
	Matchers   []Matcher      `json:"matchers"`
	Response   ResponseConfig `json:"response"`
	CreatedAt  int64          `json:"createdAt"`
	UpdatedAt  int64          `json:"updatedAt"`
}

type Matcher struct {
	Source        string      `json:"source"`
	Key           string      `json:"key"`
	Op            string      `json:"op"`
	Value         interface{} `json:"value"`
	CaseSensitive bool        `json:"caseSensitive"`
}

type ResponseConfig struct {
	Status      int               `json:"status"`
	Headers     map[string]string `json:"headers"`
	DelayMs     int               `json:"delayMs"`
	BodyType    string            `json:"bodyType"`
	Body        string            `json:"body"`
	ContentType string            `json:"contentType"`
}

type PreviewRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Query   map[string]string `json:"query"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

type PreviewResponse struct {
	Matched    bool           `json:"matched"`
	EndpointID string         `json:"endpointId,omitempty"`
	RuleID     string         `json:"ruleId,omitempty"`
	Explain    []string       `json:"explain"`
	Response   ResponseConfig `json:"response"`
}

type ExportBundle struct {
	Endpoints []Endpoint `json:"endpoints"`
	Rules     []Rule     `json:"rules"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}
