package server

import (
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

type compiledEndpoint struct {
	endpoint   Endpoint
	regex      *regexp.Regexp
	paramNames []string
	rules      []Rule
}

type RequestInfo struct {
	Method     string
	Path       string
	Query      map[string][]string
	Headers    map[string][]string
	Cookies    map[string]string
	BodyRaw    string
	BodyJSON   gjson.Result
	HasJSON    bool
	PathParams map[string]string
}

func buildIndex(endpoints map[string]Endpoint, rules map[string]Rule) map[string][]compiledEndpoint {
	byEndpoint := map[string][]Rule{}
	for _, rule := range rules {
		byEndpoint[rule.EndpointID] = append(byEndpoint[rule.EndpointID], rule)
	}
	index := map[string][]compiledEndpoint{}
	for _, ep := range endpoints {
		regex, params, err := compilePathPattern(ep.PathPattern)
		if err != nil {
			continue
		}
		compiled := compiledEndpoint{
			endpoint:   ep,
			regex:      regex,
			paramNames: params,
			rules:      byEndpoint[ep.ID],
		}
		sort.Slice(compiled.rules, func(i, j int) bool {
			if compiled.rules[i].Priority == compiled.rules[j].Priority {
				return compiled.rules[i].UpdatedAt > compiled.rules[j].UpdatedAt
			}
			return compiled.rules[i].Priority < compiled.rules[j].Priority
		})
		method := strings.ToUpper(ep.Method)
		index[method] = append(index[method], compiled)
	}
	return index
}

func compilePathPattern(pattern string) (*regexp.Regexp, []string, error) {
	if strings.HasPrefix(pattern, "re:") {
		re, err := regexp.Compile(pattern[3:])
		return re, nil, err
	}
	if pattern == "" || !strings.HasPrefix(pattern, "/") {
		return nil, nil, fmt.Errorf("pathPattern must start with /")
	}
	parts := strings.Split(strings.TrimPrefix(pattern, "/"), "/")
	params := []string{}
	reParts := make([]string, 0, len(parts))
	for i, part := range parts {
		if part == "" {
			reParts = append(reParts, "")
			continue
		}
		if part == "**" {
			if i != len(parts)-1 {
				return nil, nil, fmt.Errorf("** must be at the end")
			}
			reParts = append(reParts, ".*")
			continue
		}
		if part == "*" {
			reParts = append(reParts, "[^/]+")
			continue
		}
		if strings.HasPrefix(part, ":") {
			name := strings.TrimPrefix(part, ":")
			if !isValidParamName(name) {
				return nil, nil, fmt.Errorf("invalid param name: %s", name)
			}
			params = append(params, name)
			reParts = append(reParts, "([^/]+)")
			continue
		}
		reParts = append(reParts, regexp.QuoteMeta(part))
	}
	patternRe := "^/" + strings.Join(reParts, "/") + "$"
	re, err := regexp.Compile(patternRe)
	return re, params, err
}

func isValidParamName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if !(r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
				return false
			}
			continue
		}
		if !(r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

func (s *Store) Match(req RequestInfo) (bool, *Endpoint, *Rule, []string, ResponseConfig) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	method := strings.ToUpper(req.Method)
	candidates := s.index[method]
	explain := []string{}
	for _, compiled := range candidates {
		if !compiled.endpoint.Enabled {
			continue
		}
		matches := compiled.regex.FindStringSubmatch(req.Path)
		if matches == nil {
			continue
		}
		pathParams := map[string]string{}
		for i, name := range compiled.paramNames {
			if i+1 < len(matches) {
				pathParams[name] = matches[i+1]
			}
		}
		matchedRule, ok := matchRules(compiled.rules, req, pathParams, &explain)
		if ok {
			return true, &compiled.endpoint, matchedRule, explain, matchedRule.Response
		}
	}
	return false, nil, nil, explain, ResponseConfig{}
}

func matchRules(rules []Rule, req RequestInfo, pathParams map[string]string, explain *[]string) (*Rule, bool) {
	if len(rules) == 0 {
		return nil, false
	}
	grouped := groupByPriority(rules)
	priorities := make([]int, 0, len(grouped))
	for p := range grouped {
		priorities = append(priorities, p)
	}
	sort.Ints(priorities)
	for _, p := range priorities {
		candidates := []Rule{}
		for _, rule := range grouped[p] {
			if !rule.Enabled {
				continue
			}
			if matchAll(rule.Matchers, req, pathParams, explain) {
				candidates = append(candidates, rule)
			}
		}
		if len(candidates) == 0 {
			continue
		}
		if len(candidates) == 1 {
			return &candidates[0], true
		}
		picked := pickByWeight(candidates)
		return &picked, true
	}
	return nil, false
}

func groupByPriority(rules []Rule) map[int][]Rule {
	grouped := map[int][]Rule{}
	for _, rule := range rules {
		grouped[rule.Priority] = append(grouped[rule.Priority], rule)
	}
	return grouped
}

func pickByWeight(rules []Rule) Rule {
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	total := 0
	for _, rule := range rules {
		if rule.Weight <= 0 {
			continue
		}
		total += rule.Weight
	}
	if total == 0 {
		return rules[0]
	}
	roll := seed.Intn(total)
	for _, rule := range rules {
		if rule.Weight <= 0 {
			continue
		}
		if roll < rule.Weight {
			return rule
		}
		roll -= rule.Weight
	}
	return rules[0]
}

func matchAll(matchers []Matcher, req RequestInfo, pathParams map[string]string, explain *[]string) bool {
	for _, matcher := range matchers {
		ok, reason := matchOne(matcher, req, pathParams)
		if reason != "" {
			*explain = append(*explain, reason)
		}
		if !ok {
			return false
		}
	}
	return true
}

func matchOne(m Matcher, req RequestInfo, pathParams map[string]string) (bool, string) {
	value, exists := resolveValue(m.Source, m.Key, req, pathParams)
	if m.Op == "exists" {
		if exists {
			return true, fmt.Sprintf("%s %s exists", m.Source, m.Key)
		}
		return false, fmt.Sprintf("%s %s missing", m.Source, m.Key)
	}
	if !exists {
		return false, fmt.Sprintf("%s %s missing", m.Source, m.Key)
	}
	valueStr := value
	needle := fmt.Sprintf("%v", m.Value)
	if !m.CaseSensitive {
		valueStr = strings.ToLower(valueStr)
		needle = strings.ToLower(needle)
	}
	switch m.Op {
	case "eq":
		return valueStr == needle, fmt.Sprintf("%s %s eq %s", m.Source, m.Key, needle)
	case "ne":
		return valueStr != needle, fmt.Sprintf("%s %s ne %s", m.Source, m.Key, needle)
	case "contains":
		return strings.Contains(valueStr, needle), fmt.Sprintf("%s %s contains %s", m.Source, m.Key, needle)
	case "regex":
		re, err := regexp.Compile(needle)
		if err != nil {
			return false, fmt.Sprintf("%s %s regex invalid", m.Source, m.Key)
		}
		return re.MatchString(valueStr), fmt.Sprintf("%s %s regex %s", m.Source, m.Key, needle)
	case "in":
		switch v := m.Value.(type) {
		case []interface{}:
			for _, item := range v {
				cand := fmt.Sprintf("%v", item)
				if !m.CaseSensitive {
					cand = strings.ToLower(cand)
				}
				if valueStr == cand {
					return true, fmt.Sprintf("%s %s in", m.Source, m.Key)
				}
			}
		case []string:
			for _, item := range v {
				cand := item
				if !m.CaseSensitive {
					cand = strings.ToLower(cand)
				}
				if valueStr == cand {
					return true, fmt.Sprintf("%s %s in", m.Source, m.Key)
				}
			}
		default:
			return valueStr == needle, fmt.Sprintf("%s %s in", m.Source, m.Key)
		}
		return false, fmt.Sprintf("%s %s in miss", m.Source, m.Key)
	default:
		return false, fmt.Sprintf("%s %s op %s unsupported", m.Source, m.Key, m.Op)
	}
}

func resolveValue(source, key string, req RequestInfo, pathParams map[string]string) (string, bool) {
	switch source {
	case "method":
		return req.Method, true
	case "pathParam":
		val, ok := pathParams[key]
		return val, ok
	case "query":
		vals := req.Query[key]
		if len(vals) == 0 {
			return "", false
		}
		return vals[0], true
	case "header":
		vals := req.Headers[strings.ToLower(key)]
		if len(vals) == 0 {
			return "", false
		}
		return vals[0], true
	case "cookie":
		val, ok := req.Cookies[key]
		return val, ok
	case "bodyJsonPath":
		if !req.HasJSON {
			return "", false
		}
		result := req.BodyJSON.Get(key)
		if !result.Exists() {
			return "", false
		}
		return result.String(), true
	case "bodyRaw":
		if req.BodyRaw == "" {
			return "", false
		}
		return req.BodyRaw, true
	default:
		return "", false
	}
}
