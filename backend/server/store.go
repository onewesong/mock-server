package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Store struct {
	db *sql.DB
	mu sync.RWMutex

	endpoints map[string]Endpoint
	rules     map[string]Rule
	index     map[string][]compiledEndpoint
}

func NewStore(db *sql.DB) (*Store, error) {
	s := &Store{
		db:        db,
		endpoints: map[string]Endpoint{},
		rules:     map[string]Rule{},
		index:     map[string][]compiledEndpoint{},
	}
	if err := s.reload(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	endpoints, rules, err := loadAll(s.db)
	if err != nil {
		return err
	}

	s.endpoints = endpoints
	s.rules = rules
	s.index = buildIndex(endpoints, rules)
	return nil
}

func (s *Store) ListEndpoints() []Endpoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]Endpoint, 0, len(s.endpoints))
	for _, ep := range s.endpoints {
		items = append(items, ep)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt > items[j].UpdatedAt
	})
	return items
}

func (s *Store) GetEndpoint(id string) (Endpoint, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ep, ok := s.endpoints[id]
	return ep, ok
}

func (s *Store) CreateEndpoint(ep Endpoint) (Endpoint, error) {
	if ep.ID == "" {
		ep.ID = uuid.NewString()
	}
	now := time.Now().UnixMilli()
	ep.CreatedAt = now
	ep.UpdatedAt = now

	data, err := json.Marshal(ep.Tags)
	if err != nil {
		return Endpoint{}, err
	}

	_, err = s.db.Exec(`INSERT INTO endpoints (id, name, method, path_pattern, enabled, tags_json, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ep.ID, ep.Name, strings.ToUpper(ep.Method), ep.PathPattern, boolToInt(ep.Enabled), string(data), ep.Description, ep.CreatedAt, ep.UpdatedAt)
	if err != nil {
		return Endpoint{}, err
	}
	if err := s.reload(); err != nil {
		return Endpoint{}, err
	}
	return ep, nil
}

func (s *Store) UpdateEndpoint(id string, update Endpoint) (Endpoint, error) {
	existing, ok := s.GetEndpoint(id)
	if !ok {
		return Endpoint{}, errors.New("not found")
	}
	update.ID = existing.ID
	update.CreatedAt = existing.CreatedAt
	update.UpdatedAt = time.Now().UnixMilli()
	if update.Method == "" {
		update.Method = existing.Method
	}
	if update.PathPattern == "" {
		update.PathPattern = existing.PathPattern
	}
	data, err := json.Marshal(update.Tags)
	if err != nil {
		return Endpoint{}, err
	}
	_, err = s.db.Exec(`UPDATE endpoints SET name=?, method=?, path_pattern=?, enabled=?, tags_json=?, description=?, updated_at=? WHERE id=?`,
		update.Name, strings.ToUpper(update.Method), update.PathPattern, boolToInt(update.Enabled), string(data), update.Description, update.UpdatedAt, update.ID)
	if err != nil {
		return Endpoint{}, err
	}
	if err := s.reload(); err != nil {
		return Endpoint{}, err
	}
	return update, nil
}

func (s *Store) DeleteEndpoint(id string) error {
	_, err := s.db.Exec(`DELETE FROM endpoints WHERE id=?`, id)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM rules WHERE endpoint_id=?`, id)
	if err != nil {
		return err
	}
	return s.reload()
}

func (s *Store) ListRules(endpointID string) []Rule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := []Rule{}
	for _, rule := range s.rules {
		if rule.EndpointID == endpointID {
			items = append(items, rule)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Priority == items[j].Priority {
			return items[i].UpdatedAt > items[j].UpdatedAt
		}
		return items[i].Priority < items[j].Priority
	})
	return items
}

func (s *Store) GetRule(id string) (Rule, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rule, ok := s.rules[id]
	return rule, ok
}

func (s *Store) CreateRule(rule Rule) (Rule, error) {
	if rule.ID == "" {
		rule.ID = uuid.NewString()
	}
	now := time.Now().UnixMilli()
	rule.CreatedAt = now
	rule.UpdatedAt = now
	if rule.Weight == 0 {
		rule.Weight = 1
	}
	matchersJSON, err := json.Marshal(rule.Matchers)
	if err != nil {
		return Rule{}, err
	}
	responseJSON, err := json.Marshal(rule.Response)
	if err != nil {
		return Rule{}, err
	}
	_, err = s.db.Exec(`INSERT INTO rules (id, endpoint_id, name, enabled, priority, weight, matchers_json, response_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.ID, rule.EndpointID, rule.Name, boolToInt(rule.Enabled), rule.Priority, rule.Weight, string(matchersJSON), string(responseJSON), rule.CreatedAt, rule.UpdatedAt)
	if err != nil {
		return Rule{}, err
	}
	if err := s.reload(); err != nil {
		return Rule{}, err
	}
	return rule, nil
}

func (s *Store) UpdateRule(id string, update Rule) (Rule, error) {
	existing, ok := s.GetRule(id)
	if !ok {
		return Rule{}, errors.New("not found")
	}
	update.ID = existing.ID
	update.EndpointID = existing.EndpointID
	update.CreatedAt = existing.CreatedAt
	update.UpdatedAt = time.Now().UnixMilli()
	if update.Weight == 0 {
		update.Weight = existing.Weight
	}
	matchersJSON, err := json.Marshal(update.Matchers)
	if err != nil {
		return Rule{}, err
	}
	responseJSON, err := json.Marshal(update.Response)
	if err != nil {
		return Rule{}, err
	}
	_, err = s.db.Exec(`UPDATE rules SET name=?, enabled=?, priority=?, weight=?, matchers_json=?, response_json=?, updated_at=? WHERE id=?`,
		update.Name, boolToInt(update.Enabled), update.Priority, update.Weight, string(matchersJSON), string(responseJSON), update.UpdatedAt, update.ID)
	if err != nil {
		return Rule{}, err
	}
	if err := s.reload(); err != nil {
		return Rule{}, err
	}
	return update, nil
}

func (s *Store) DeleteRule(id string) error {
	_, err := s.db.Exec(`DELETE FROM rules WHERE id=?`, id)
	if err != nil {
		return err
	}
	return s.reload()
}

func (s *Store) ExportAll() (ExportBundle, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bundle := ExportBundle{}
	for _, ep := range s.endpoints {
		bundle.Endpoints = append(bundle.Endpoints, ep)
	}
	for _, rule := range s.rules {
		bundle.Rules = append(bundle.Rules, rule)
	}
	return bundle, nil
}

func (s *Store) ImportAll(bundle ExportBundle) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM rules`); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`DELETE FROM endpoints`); err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, ep := range bundle.Endpoints {
		data, err := json.Marshal(ep.Tags)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		_, err = tx.Exec(`INSERT INTO endpoints (id, name, method, path_pattern, enabled, tags_json, description, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			ep.ID, ep.Name, strings.ToUpper(ep.Method), ep.PathPattern, boolToInt(ep.Enabled), string(data), ep.Description, ep.CreatedAt, ep.UpdatedAt)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	for _, rule := range bundle.Rules {
		matchersJSON, err := json.Marshal(rule.Matchers)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		responseJSON, err := json.Marshal(rule.Response)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		_, err = tx.Exec(`INSERT INTO rules (id, endpoint_id, name, enabled, priority, weight, matchers_json, response_json, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			rule.ID, rule.EndpointID, rule.Name, boolToInt(rule.Enabled), rule.Priority, rule.Weight, string(matchersJSON), string(responseJSON), rule.CreatedAt, rule.UpdatedAt)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.reload()
}

func loadAll(db *sql.DB) (map[string]Endpoint, map[string]Rule, error) {
	endpoints := map[string]Endpoint{}
	rules := map[string]Rule{}

	rows, err := db.Query(`SELECT id, name, method, path_pattern, enabled, tags_json, description, created_at, updated_at FROM endpoints`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ep Endpoint
		var enabled int
		var tagsJSON string
		if err := rows.Scan(&ep.ID, &ep.Name, &ep.Method, &ep.PathPattern, &enabled, &tagsJSON, &ep.Description, &ep.CreatedAt, &ep.UpdatedAt); err != nil {
			return nil, nil, err
		}
		ep.Enabled = enabled == 1
		if tagsJSON != "" {
			if err := json.Unmarshal([]byte(tagsJSON), &ep.Tags); err != nil {
				return nil, nil, err
			}
		}
		endpoints[ep.ID] = ep
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	ruleRows, err := db.Query(`SELECT id, endpoint_id, name, enabled, priority, weight, matchers_json, response_json, created_at, updated_at FROM rules`)
	if err != nil {
		return nil, nil, err
	}
	defer ruleRows.Close()
	for ruleRows.Next() {
		var rule Rule
		var enabled int
		var matchersJSON string
		var responseJSON string
		if err := ruleRows.Scan(&rule.ID, &rule.EndpointID, &rule.Name, &enabled, &rule.Priority, &rule.Weight, &matchersJSON, &responseJSON, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, nil, err
		}
		rule.Enabled = enabled == 1
		if matchersJSON != "" {
			if err := json.Unmarshal([]byte(matchersJSON), &rule.Matchers); err != nil {
				return nil, nil, fmt.Errorf("parse matchers %s: %w", rule.ID, err)
			}
		}
		if responseJSON != "" {
			if err := json.Unmarshal([]byte(responseJSON), &rule.Response); err != nil {
				return nil, nil, fmt.Errorf("parse response %s: %w", rule.ID, err)
			}
		}
		rules[rule.ID] = rule
	}
	if err := ruleRows.Err(); err != nil {
		return nil, nil, err
	}
	return endpoints, rules, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
