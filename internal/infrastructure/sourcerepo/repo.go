package sourcerepo

import (
	_ "embed"
	"fmt"

	"encoding/json/v2"

	"github.com/gitrus/digikeeper-log/internal/domain/model"
)

//go:embed sources.json
var sourcesRaw []byte

// Repo is a JSON-backed repository for client source mappings.
type Repo struct {
	byName map[string]model.Source
	byID   map[int]model.Source
}

func New() *Repo {
	var raw map[string]int
	if err := json.Unmarshal(sourcesRaw, &raw); err != nil {
		panic(fmt.Sprintf("sourcerepo: unmarshal sources.json: %v", err))
	}
	byName := make(map[string]model.Source, len(raw))
	byID := make(map[int]model.Source, len(raw))
	for name, id := range raw {
		s := model.Source{ID: id, Name: name}
		byName[name] = s
		byID[id] = s
	}
	return &Repo{byName: byName, byID: byID}
}

func (r *Repo) ResolveID(clientName string) int {
	return r.byName[clientName].ID
}

func (r *Repo) ResolveName(id int) string {
	return r.byID[id].Name
}
