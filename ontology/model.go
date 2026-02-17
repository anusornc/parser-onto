package ontology

// Ontology represents a parsed ChEBI ontology.
type Ontology struct {
	FormatVersion string    `json:"format_version,omitempty"`
	DataVersion   string    `json:"data_version,omitempty"`
	Ontology      string    `json:"ontology,omitempty"`
	Terms         []Term    `json:"terms"`
	TypeDefs      []TypeDef `json:"typedefs,omitempty"`
}

// TypeDef represents an OBO Typedef stanza (object property).
type TypeDef struct {
	ID           string `json:"id"`
	Name         string `json:"name,omitempty"`
	IsTransitive bool   `json:"is_transitive,omitempty"`
	IsReflexive  bool   `json:"is_reflexive,omitempty"`
}

// IntersectionPart represents one part of an intersection_of definition.
// If Relationship is empty, it's a genus (plain class). Otherwise it's
// a differentia: ∃Relationship.TargetID.
type IntersectionPart struct {
	Relationship string `json:"relationship,omitempty"`
	TargetID     string `json:"target_id"`
}

// Term represents a single ChEBI ontology term (chemical entity).
type Term struct {
	ID            string            `json:"id"`
	Name          string            `json:"name,omitempty"`
	Namespace     string            `json:"namespace,omitempty"`
	Definition    string            `json:"definition,omitempty"`
	IsObsolete    bool              `json:"is_obsolete,omitempty"`
	Comment       string            `json:"comment,omitempty"`
	Subsets       []string          `json:"subsets,omitempty"`
	Synonyms      []Synonym         `json:"synonyms,omitempty"`
	Xrefs         []string          `json:"xrefs,omitempty"`
	AltIDs        []string          `json:"alt_ids,omitempty"`
	Relationships  []Relationship    `json:"relationships,omitempty"`
	IntersectionOf []IntersectionPart `json:"intersection_of,omitempty"`
	Properties     map[string]string `json:"properties,omitempty"`
}

// Synonym represents a term synonym with its scope type.
type Synonym struct {
	Text  string   `json:"text"`
	Scope string   `json:"scope"` // EXACT, BROAD, NARROW, RELATED
	Type  string   `json:"type,omitempty"`
	Xrefs []string `json:"xrefs,omitempty"`
}

// Relationship represents a typed relationship to another term.
type Relationship struct {
	Type     string `json:"type"`      // is_a, has_part, has_role, etc.
	TargetID string `json:"target_id"`
	Name     string `json:"name,omitempty"`
}
