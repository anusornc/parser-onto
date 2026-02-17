package reasoner

import (
	"encoding/json"
	"io"
	"time"
)

// Taxonomy holds the classified hierarchy after transitive reduction.
type Taxonomy struct {
	DirectParents  [][]ConceptID
	DirectChildren [][]ConceptID
}

// BuildTaxonomy extracts the direct (non-redundant) subsumption hierarchy
// from saturated contexts by performing transitive reduction.
func BuildTaxonomy(contexts []Context, st *SymbolTable) *Taxonomy {
	n := st.ConceptCount()
	tax := &Taxonomy{
		DirectParents:  make([][]ConceptID, n),
		DirectChildren: make([][]ConceptID, n),
	}

	for c := ConceptID(2); c < ConceptID(n); c++ {
		supers := contexts[c].superSet
		if len(supers) == 0 {
			continue
		}

		// Collect candidate parents (everything in S(C) except C itself and Top).
		candidates := make([]ConceptID, 0, len(supers))
		hasTop := false
		for s := range supers {
			if s == c {
				continue
			}
			if s == Top {
				hasTop = true
				continue
			}
			if s == Bottom {
				continue
			}
			candidates = append(candidates, s)
		}

		// Transitive reduction: B is a direct parent of C iff no other
		// candidate S also subsumes B (i.e., B ∈ S(S)).
		direct := make([]ConceptID, 0, 4)
		for _, b := range candidates {
			isDirect := true
			for _, s := range candidates {
				if s == b {
					continue
				}
				if _, ok := contexts[s].superSet[b]; ok {
					isDirect = false
					break
				}
			}
			if isDirect {
				direct = append(direct, b)
			}
		}

		// If no direct parents found but Top was in S(C), Top is the direct parent.
		if len(direct) == 0 && hasTop {
			direct = append(direct, Top)
		}

		tax.DirectParents[c] = direct
		for _, p := range direct {
			tax.DirectChildren[p] = append(tax.DirectChildren[p], c)
		}
	}

	return tax
}

// ClassifiedConcept represents a concept in the classified hierarchy.
type ClassifiedConcept struct {
	ID             string   `json:"id"`
	Name           string   `json:"name,omitempty"`
	DirectParents  []string `json:"direct_parents"`
	DirectChildren []string `json:"direct_children,omitempty"`
}

// ClassificationStats holds timing and size metrics.
type ClassificationStats struct {
	ConceptCount         int    `json:"concept_count"`
	RoleCount            int    `json:"role_count"`
	InferredSubsumptions int    `json:"inferred_subsumptions"`
	ParseTimeMs          int64  `json:"parse_time_ms"`
	NormalizeTimeMs      int64  `json:"normalize_time_ms"`
	SaturateTimeMs       int64  `json:"saturate_time_ms"`
	ReductionTimeMs      int64  `json:"reduction_time_ms"`
	TotalTimeMs          int64  `json:"total_time_ms"`
}

// ClassifiedHierarchy is the top-level JSON output.
type ClassifiedHierarchy struct {
	Concepts []ClassifiedConcept `json:"concepts"`
	Stats    ClassificationStats `json:"stats"`
}

// ToJSON converts the taxonomy to a ClassifiedHierarchy for JSON output.
func (tax *Taxonomy) ToJSON(contexts []Context, st *SymbolTable, stats ClassificationStats) *ClassifiedHierarchy {
	result := &ClassifiedHierarchy{
		Stats: stats,
	}

	// Count inferred subsumptions (total S(C) entries beyond self and Top).
	inferred := 0
	for c := ConceptID(2); c < ConceptID(st.ConceptCount()); c++ {
		// Only count named concepts (non-empty name in symbol table).
		name := st.ConceptName(c)
		if name == "" {
			continue
		}
		inferred += len(contexts[c].superSet) - 2 // subtract self and Top
		if inferred < 0 {
			inferred = 0
		}
	}
	result.Stats.InferredSubsumptions = inferred

	// Build concept list (only named concepts).
	for c := ConceptID(2); c < ConceptID(st.ConceptCount()); c++ {
		name := st.ConceptName(c)
		if name == "" {
			continue // skip fresh/anonymous concepts
		}

		cc := ClassifiedConcept{
			ID:            name,
			DirectParents: make([]string, 0, len(tax.DirectParents[c])),
		}

		for _, p := range tax.DirectParents[c] {
			pname := st.ConceptName(p)
			if pname != "" {
				cc.DirectParents = append(cc.DirectParents, pname)
			}
		}

		if len(tax.DirectChildren[c]) > 0 {
			cc.DirectChildren = make([]string, 0, len(tax.DirectChildren[c]))
			for _, ch := range tax.DirectChildren[c] {
				chname := st.ConceptName(ch)
				if chname != "" {
					cc.DirectChildren = append(cc.DirectChildren, chname)
				}
			}
		}

		result.Concepts = append(result.Concepts, cc)
	}

	return result
}

// WriteClassifiedJSON writes the classified hierarchy as JSON.
func WriteClassifiedJSON(w io.Writer, hierarchy *ClassifiedHierarchy) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(hierarchy)
}

// MakeStats creates a ClassificationStats from timing durations.
func MakeStats(st *SymbolTable, parseTime, normTime, satTime, redTime time.Duration) ClassificationStats {
	total := parseTime + normTime + satTime + redTime
	return ClassificationStats{
		ConceptCount:    st.ConceptCount() - 2, // exclude Top and Bottom
		RoleCount:       st.RoleCount(),
		ParseTimeMs:     parseTime.Milliseconds(),
		NormalizeTimeMs: normTime.Milliseconds(),
		SaturateTimeMs:  satTime.Milliseconds(),
		ReductionTimeMs: redTime.Milliseconds(),
		TotalTimeMs:     total.Milliseconds(),
	}
}
