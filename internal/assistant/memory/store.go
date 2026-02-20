package memory

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Entity represents a knowledge graph node.
// Data format is compatible with MCP Memory Server.
type Entity struct {
	Name         string   `json:"name"`
	EntityType   string   `json:"entityType"`
	Observations []string `json:"observations"`
}

// Relation represents a directed edge between two entities.
// RelationType should use active voice (e.g., "owns", "depends_on").
type Relation struct {
	From         string `json:"from"`
	To           string `json:"to"`
	RelationType string `json:"relationType"`
}

// record is the on-disk JSONL wrapper — one record per line.
type record struct {
	Type string `json:"type"`
	// Inlined fields for both entity and relation.
	Name         string   `json:"name,omitempty"`
	EntityType   string   `json:"entityType,omitempty"`
	Observations []string `json:"observations,omitempty"`
	From         string   `json:"from,omitempty"`
	To           string   `json:"to,omitempty"`
	RelationType string   `json:"relationType,omitempty"`
}

// memoryDir returns ~/.codes/assistant/ and creates it if needed.
func memoryDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".codes", "assistant")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// memoryPath returns the full path to memory.jsonl.
func memoryPath() (string, error) {
	dir, err := memoryDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "memory.jsonl"), nil
}

// readAll reads the JSONL file and returns all entities and relations.
// Returns empty slices if the file does not exist.
func readAll() ([]Entity, []Relation, error) {
	path, err := memoryPath()
	if err != nil {
		return nil, nil, err
	}

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("open memory: %w", err)
	}
	defer f.Close()

	var entities []Entity
	var relations []Relation

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var r record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			// Skip malformed lines — be resilient.
			continue
		}
		switch r.Type {
		case "entity":
			obs := r.Observations
			if obs == nil {
				obs = []string{}
			}
			entities = append(entities, Entity{
				Name:         r.Name,
				EntityType:   r.EntityType,
				Observations: obs,
			})
		case "relation":
			relations = append(relations, Relation{
				From:         r.From,
				To:           r.To,
				RelationType: r.RelationType,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("scan memory: %w", err)
	}
	return entities, relations, nil
}

// writeAll persists entities and relations atomically via tmpfile + rename.
func writeAll(entities []Entity, relations []Relation) error {
	path, err := memoryPath()
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create tmp memory: %w", err)
	}

	enc := json.NewEncoder(f)
	for _, e := range entities {
		r := record{
			Type:         "entity",
			Name:         e.Name,
			EntityType:   e.EntityType,
			Observations: e.Observations,
		}
		if err := enc.Encode(r); err != nil {
			f.Close()
			os.Remove(tmp)
			return fmt.Errorf("encode entity: %w", err)
		}
	}
	for _, rel := range relations {
		r := record{
			Type:         "relation",
			From:         rel.From,
			To:           rel.To,
			RelationType: rel.RelationType,
		}
		if err := enc.Encode(r); err != nil {
			f.Close()
			os.Remove(tmp)
			return fmt.Errorf("encode relation: %w", err)
		}
	}

	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("close tmp memory: %w", err)
	}
	return os.Rename(tmp, path)
}

// CreateEntities creates entities in bulk, deduplicating by name.
// Existing entities are not overwritten.
func CreateEntities(entities []Entity) error {
	existing, relations, err := readAll()
	if err != nil {
		return err
	}

	// Build a set of existing names.
	seen := make(map[string]struct{}, len(existing))
	for _, e := range existing {
		seen[e.Name] = struct{}{}
	}

	for _, e := range entities {
		if _, ok := seen[e.Name]; ok {
			continue
		}
		obs := e.Observations
		if obs == nil {
			obs = []string{}
		}
		existing = append(existing, Entity{
			Name:         e.Name,
			EntityType:   e.EntityType,
			Observations: obs,
		})
		seen[e.Name] = struct{}{}
	}

	return writeAll(existing, relations)
}

// AddObservations appends observations to an existing entity.
// Returns an error if the entity is not found.
func AddObservations(name string, observations []string) error {
	entities, relations, err := readAll()
	if err != nil {
		return err
	}

	found := false
	for i := range entities {
		if entities[i].Name == name {
			// Append only new observations (deduplicate).
			existing := make(map[string]struct{}, len(entities[i].Observations))
			for _, o := range entities[i].Observations {
				existing[o] = struct{}{}
			}
			for _, o := range observations {
				if _, ok := existing[o]; !ok {
					entities[i].Observations = append(entities[i].Observations, o)
				}
			}
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("entity %q not found", name)
	}

	return writeAll(entities, relations)
}

// CreateRelations creates relations in bulk, deduplicating by from+to+type.
func CreateRelations(relations []Relation) error {
	entities, existing, err := readAll()
	if err != nil {
		return err
	}

	type key struct{ from, to, relType string }
	seen := make(map[key]struct{}, len(existing))
	for _, r := range existing {
		seen[key{r.From, r.To, r.RelationType}] = struct{}{}
	}

	for _, r := range relations {
		k := key{r.From, r.To, r.RelationType}
		if _, ok := seen[k]; ok {
			continue
		}
		existing = append(existing, r)
		seen[k] = struct{}{}
	}

	return writeAll(entities, existing)
}

// SearchNodes performs a case-insensitive substring search over entity names,
// entity types, and observations. Returns matching entities.
func SearchNodes(query string) ([]Entity, error) {
	entities, _, err := readAll()
	if err != nil {
		return nil, err
	}

	q := strings.ToLower(query)
	var results []Entity
	for _, e := range entities {
		if matchesQuery(e, q) {
			results = append(results, e)
		}
	}
	return results, nil
}

// matchesQuery returns true if the entity matches the lowercase query string.
func matchesQuery(e Entity, q string) bool {
	if strings.Contains(strings.ToLower(e.Name), q) {
		return true
	}
	if strings.Contains(strings.ToLower(e.EntityType), q) {
		return true
	}
	for _, o := range e.Observations {
		if strings.Contains(strings.ToLower(o), q) {
			return true
		}
	}
	return false
}

// DeleteEntity removes an entity by name and all relations where it appears as
// either the source (from) or the target (to).
func DeleteEntity(name string) error {
	entities, relations, err := readAll()
	if err != nil {
		return err
	}

	filtered := entities[:0]
	for _, e := range entities {
		if e.Name != name {
			filtered = append(filtered, e)
		}
	}

	filteredRel := relations[:0]
	for _, r := range relations {
		if r.From != name && r.To != name {
			filteredRel = append(filteredRel, r)
		}
	}

	return writeAll(filtered, filteredRel)
}

// LoadGraph reads the complete knowledge graph from disk.
// Intended for injecting into the system prompt.
func LoadGraph() ([]Entity, []Relation, error) {
	return readAll()
}
