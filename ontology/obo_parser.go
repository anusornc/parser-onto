package ontology

import (
	"bufio"
	"io"
	"strings"
)

const (
	initialTermCapacity = 200000 // ChEBI has ~180k terms
	scannerBufferSize   = 1 << 20 // 1 MB
)

// internPool avoids duplicate string allocations for repeated values.
type internPool struct {
	m map[string]string
}

func newInternPool() *internPool {
	return &internPool{m: make(map[string]string, 64)}
}

func (p *internPool) get(s string) string {
	if v, ok := p.m[s]; ok {
		return v
	}
	p.m[s] = s
	return s
}

// ParseOBO parses a ChEBI OBO-format ontology from the given reader.
func ParseOBO(r io.Reader) (*Ontology, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, scannerBufferSize), scannerBufferSize)

	ont := &Ontology{
		Terms: make([]Term, 0, initialTermCapacity),
	}
	pool := newInternPool()

	// Parse header
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if line == "[Term]" {
			term := parseTerm(scanner, pool)
			ont.Terms = append(ont.Terms, term)
			break
		}
		if line[0] == '[' {
			// Skip non-Term stanzas in header area
			break
		}
		parseHeaderLine(ont, line)
	}

	// Parse remaining stanzas
	for scanner.Scan() {
		line := scanner.Text()
		switch line {
		case "[Term]":
			term := parseTerm(scanner, pool)
			ont.Terms = append(ont.Terms, term)
		case "[Typedef]":
			td := parseTypeDef(scanner, pool)
			ont.TypeDefs = append(ont.TypeDefs, td)
		}
		// Skip other stanza types
	}

	return ont, scanner.Err()
}

func parseHeaderLine(ont *Ontology, line string) {
	key, val, ok := strings.Cut(line, ": ")
	if !ok {
		return
	}
	switch key {
	case "format-version":
		ont.FormatVersion = val
	case "data-version":
		ont.DataVersion = val
	case "ontology":
		ont.Ontology = val
	}
}

func parseTerm(scanner *bufio.Scanner, pool *internPool) Term {
	var t Term
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break // End of stanza
		}

		key, val, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}

		switch key {
		case "id":
			t.ID = val
		case "name":
			t.Name = val
		case "namespace":
			t.Namespace = pool.get(val)
		case "def":
			t.Definition = parseQuoted(val)
		case "comment":
			t.Comment = val
		case "subset":
			t.Subsets = append(t.Subsets, pool.get(val))
		case "synonym":
			t.Synonyms = append(t.Synonyms, parseSynonym(val))
		case "xref":
			t.Xrefs = append(t.Xrefs, val)
		case "alt_id":
			t.AltIDs = append(t.AltIDs, val)
		case "is_a":
			rel := parseIsA(val, pool)
			t.Relationships = append(t.Relationships, rel)
		case "relationship":
			rel := parseRelationship(val, pool)
			t.Relationships = append(t.Relationships, rel)
		case "intersection_of":
			t.IntersectionOf = append(t.IntersectionOf, parseIntersectionOf(val, pool))
		case "is_obsolete":
			t.IsObsolete = val == "true"
		case "property_value":
			k, v := parsePropertyValue(val)
			if k != "" {
				if t.Properties == nil {
					t.Properties = make(map[string]string, 4)
				}
				t.Properties[k] = v
			}
		}
	}
	return t
}

// parseQuoted extracts text between the first pair of double quotes.
func parseQuoted(s string) string {
	start := strings.IndexByte(s, '"')
	if start < 0 {
		return s
	}
	start++
	end := strings.IndexByte(s[start:], '"')
	if end < 0 {
		return s[start:]
	}
	return s[start : start+end]
}

// parseSynonym parses: "text" SCOPE [xrefs]
func parseSynonym(s string) Synonym {
	var syn Synonym
	syn.Text = parseQuoted(s)

	// Find scope after closing quote
	afterQuote := strings.IndexByte(s[1:], '"')
	if afterQuote < 0 {
		return syn
	}
	rest := s[afterQuote+3:] // skip past closing quote and space

	// Scope is the first word
	parts := strings.Fields(rest)
	if len(parts) > 0 {
		syn.Scope = parts[0]
	}
	if len(parts) > 1 && !strings.HasPrefix(parts[1], "[") {
		syn.Type = parts[1]
	}

	// Extract xrefs from brackets
	bracketStart := strings.IndexByte(rest, '[')
	bracketEnd := strings.LastIndexByte(rest, ']')
	if bracketStart >= 0 && bracketEnd > bracketStart+1 {
		xrefStr := rest[bracketStart+1 : bracketEnd]
		if xrefStr != "" {
			syn.Xrefs = strings.Split(xrefStr, ", ")
		}
	}

	return syn
}

// parseIsA parses: "CHEBI:12345 ! name"
func parseIsA(val string, pool *internPool) Relationship {
	rel := Relationship{Type: pool.get("is_a")}
	id, name, _ := strings.Cut(val, " ! ")
	rel.TargetID = id
	rel.Name = name
	return rel
}

// parseRelationship parses: "type CHEBI:12345 ! name"
func parseRelationship(val string, pool *internPool) Relationship {
	var rel Relationship
	parts := strings.SplitN(val, " ", 3)
	if len(parts) >= 1 {
		rel.Type = pool.get(parts[0])
	}
	if len(parts) >= 2 {
		idAndName := parts[1]
		if len(parts) == 3 {
			idAndName = parts[1] + " " + parts[2]
		}
		id, name, _ := strings.Cut(idAndName, " ! ")
		rel.TargetID = id
		rel.Name = name
	}
	return rel
}

// parseIntersectionOf parses: "CHEBI:12345" (genus) or "relationship CHEBI:12345" (differentia).
func parseIntersectionOf(val string, pool *internPool) IntersectionPart {
	// Strip trailing comment
	v, _, _ := strings.Cut(val, " ! ")
	v = strings.TrimSpace(v)

	parts := strings.SplitN(v, " ", 2)
	if len(parts) == 1 {
		// Genus: just a class ID
		return IntersectionPart{TargetID: parts[0]}
	}
	// Differentia: relationship target
	return IntersectionPart{
		Relationship: pool.get(parts[0]),
		TargetID:     parts[1],
	}
}

// parseTypeDef parses a [Typedef] stanza.
func parseTypeDef(scanner *bufio.Scanner, pool *internPool) TypeDef {
	var td TypeDef
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break
		}
		key, val, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		switch key {
		case "id":
			td.ID = pool.get(val)
		case "name":
			td.Name = val
		case "is_transitive":
			td.IsTransitive = val == "true"
		case "is_reflexive":
			td.IsReflexive = val == "true"
		}
	}
	return td
}

// parsePropertyValue parses: "key value xsd:type" or "key \"value\" xsd:type"
func parsePropertyValue(val string) (string, string) {
	parts := strings.SplitN(val, " ", 3)
	if len(parts) < 2 {
		return "", ""
	}
	key := parts[0]
	if strings.HasPrefix(parts[1], "\"") {
		// Quoted value
		return key, parseQuoted(val)
	}
	return key, parts[1]
}
