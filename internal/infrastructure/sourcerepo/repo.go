package sourcerepo

import (
	_ "embed"
	"fmt"

	"github.com/gitrus/digikeeper-log/internal/jsonx"
)

//go:embed sources.json
var sourcesRaw []byte

type source struct {
	ID   int
	Name string
}

// Repo is a JSON-backed repository for client source mappings.
type Repo struct {
	byName map[string]source
	byID   map[int]source
}

func New() *Repo {
	var raw map[string]int
	if err := jsonx.Unmarshal(sourcesRaw, &raw); err != nil {
		panic(fmt.Sprintf("sourcerepo: unmarshal sources.json: %v", err))
	}
	byName := make(map[string]source, len(raw))
	byID := make(map[int]source, len(raw))
	for name, id := range raw {
		s := source{ID: id, Name: name}
		byName[name] = s
		byID[id] = s
	}
	return &Repo{byName: byName, byID: byID}
}

// ResolveID return id of client by name
func (r *Repo) ResolveID(clientName string) int {
	return r.byName[clientName].ID
}

// ResolveID return name of client by id
func (r *Repo) ResolveName(id int) string {
	return r.byID[id].Name
}
