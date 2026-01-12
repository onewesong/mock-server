package server

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
)

var (
	allowedMethods = map[string]bool{
		"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true, "HEAD": true, "OPTIONS": true,
	}
)

func validateEndpoint(ep Endpoint) error {
	method := strings.ToUpper(ep.Method)
	if method == "" || !allowedMethods[method] {
		return fmt.Errorf("invalid method")
	}
	if ep.PathPattern == "" || !strings.HasPrefix(ep.PathPattern, "/") {
		return fmt.Errorf("pathPattern must start with /")
	}
	if strings.HasPrefix(ep.PathPattern, "/__admin") || strings.HasPrefix(ep.PathPattern, "/_") {
		return fmt.Errorf("pathPattern must not start with /__admin or /_")
	}
	if strings.HasPrefix(ep.PathPattern, "re:") {
		if _, err := regexp.Compile(ep.PathPattern[3:]); err != nil {
			return fmt.Errorf("pathPattern regex invalid")
		}
		return nil
	}
	_, _, err := compilePathPattern(ep.PathPattern)
	if err != nil {
		return err
	}
	return nil
}

func validateRule(rule Rule) error {
	if rule.EndpointID == "" {
		return errors.New("endpointId required")
	}
	if rule.Priority < 0 {
		return errors.New("priority must be >= 0")
	}
	if rule.Weight < 0 {
		return errors.New("weight must be >= 0")
	}
	for _, m := range rule.Matchers {
		if err := validateMatcher(m); err != nil {
			return err
		}
	}
	return validateResponse(rule.Response)
}

func validateMatcher(m Matcher) error {
	sources := map[string]bool{
		"pathParam": true, "query": true, "header": true, "cookie": true, "bodyJsonPath": true, "bodyRaw": true, "method": true,
	}
	if !sources[m.Source] {
		return fmt.Errorf("matcher source invalid")
	}
	ops := map[string]bool{"eq": true, "ne": true, "contains": true, "regex": true, "in": true, "exists": true}
	if !ops[m.Op] {
		return fmt.Errorf("matcher op invalid")
	}
	if m.Op == "regex" {
		if _, err := regexp.Compile(fmt.Sprintf("%v", m.Value)); err != nil {
			return fmt.Errorf("matcher regex invalid")
		}
	}
	return nil
}

func validateResponse(resp ResponseConfig) error {
	if resp.Status != 0 {
		if resp.Status < 100 || resp.Status > 599 {
			return fmt.Errorf("status invalid")
		}
	}
	if resp.BodyType != "" && resp.BodyType != "json" && resp.BodyType != "text" {
		return fmt.Errorf("bodyType invalid")
	}
	if resp.BodyType == "json" && strings.TrimSpace(resp.Body) != "" {
		if !gjson.Valid(resp.Body) {
			return fmt.Errorf("body json invalid")
		}
	}
	return nil
}
