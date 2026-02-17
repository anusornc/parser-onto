package reasoner

import (
	"github.com/nodeadmin/chebi-parser/ontology"
)

// Normalize converts a parsed ontology into a SymbolTable and AxiomStore
// suitable for EL saturation. It extracts all axioms from the parsed terms
// and normalizes them into the six canonical forms.
func Normalize(ont *ontology.Ontology) (*SymbolTable, *AxiomStore) {
	st := NewSymbolTable()

	// First pass: register all concept and role IDs.
	for i := range ont.Terms {
		t := &ont.Terms[i]
		if t.IsObsolete {
			continue
		}
		st.InternConcept(t.ID)
		for _, rel := range t.Relationships {
			if rel.Type != "is_a" {
				st.InternRole(rel.Type)
			}
			st.InternConcept(rel.TargetID)
		}
	}

	// Register roles from TypeDefs and their properties.
	for i := range ont.TypeDefs {
		st.InternRole(ont.TypeDefs[i].ID)
	}

	// Second pass: create axiom store and populate it.
	store := NewAxiomStore(st)

	// Set role properties from TypeDefs.
	for i := range ont.TypeDefs {
		td := &ont.TypeDefs[i]
		rid := st.InternRole(td.ID)
		if td.IsTransitive {
			store.SetTransitive(rid)
		}
		if td.IsReflexive {
			store.SetReflexive(rid)
		}
	}

	// Extract axioms from terms.
	for i := range ont.Terms {
		t := &ont.Terms[i]
		if t.IsObsolete {
			continue
		}
		cid := st.InternConcept(t.ID)

		for _, rel := range t.Relationships {
			targetID := st.InternConcept(rel.TargetID)

			if rel.Type == "is_a" {
				// NF1: C ⊑ Target
				store.AddSubsumption(cid, targetID)
			} else {
				// NF3: C ⊑ ∃R.Target
				rid := st.InternRole(rel.Type)
				store.AddExistRight(cid, rid, targetID)
			}
		}

		// Handle intersection_of (conjunction / equivalentClass).
		// In OBO: intersection_of lines define an equivalence:
		//   C ≡ A₁ ⊓ A₂ ⊓ ... ⊓ ∃R.B ⊓ ...
		// This decomposes to:
		//   C ⊑ A₁, C ⊑ A₂, C ⊑ ∃R.B (already handled by is_a/relationship)
		//   A₁ ⊓ A₂ ⊓ ... ⊑ C (GCI conjunctions)
		if len(t.IntersectionOf) > 0 {
			normalizeIntersection(st, store, cid, t.IntersectionOf)
		}
	}

	// Grow store to accommodate any fresh concepts created during normalization.
	store.Grow(st.ConceptCount())
	store.GrowRoles(st.RoleCount())

	return st, store
}

// normalizeIntersection handles intersection_of axioms (equivalence decomposition).
// The forward direction (C ⊑ each conjunct) is already handled by is_a/relationship.
// This adds the reverse: conjunct₁ ⊓ conjunct₂ ⊓ ... ⊑ C.
func normalizeIntersection(st *SymbolTable, store *AxiomStore, cid ConceptID, parts []ontology.IntersectionPart) {
	// Collect the concept IDs for each conjunct.
	// For genus (plain class), it's the class ID directly.
	// For differentia (∃R.F), create a fresh concept X, add ∃R.F ⊑ X (NF4).
	conjuncts := make([]ConceptID, 0, len(parts))

	for _, part := range parts {
		if part.Relationship == "" {
			// Genus: plain concept
			conjuncts = append(conjuncts, st.InternConcept(part.TargetID))
		} else {
			// Differentia: ∃R.F — introduce fresh concept X, add NF4: ∃R.F ⊑ X
			rid := st.InternRole(part.Relationship)
			fill := st.InternConcept(part.TargetID)
			fresh := st.FreshConcept()
			store.Grow(st.ConceptCount())
			store.AddExistLeft(rid, fill, fresh)
			conjuncts = append(conjuncts, fresh)
		}
	}

	// Now build the binary conjunction tree: conjuncts[0] ⊓ conjuncts[1] ⊓ ... ⊑ C
	if len(conjuncts) == 0 {
		return
	}
	if len(conjuncts) == 1 {
		store.AddSubsumption(conjuncts[0], cid)
		return
	}

	// Binary decomposition: ((c0 ⊓ c1) ⊓ c2) ⊓ ... ⊑ C
	// Introduce fresh concepts for intermediate conjunctions.
	acc := conjuncts[0]
	for i := 1; i < len(conjuncts); i++ {
		var result ConceptID
		if i == len(conjuncts)-1 {
			result = cid // final step targets the original concept
		} else {
			result = st.FreshConcept()
			store.Grow(st.ConceptCount())
		}
		store.AddConjunction(acc, conjuncts[i], result)
		acc = result
	}
}
