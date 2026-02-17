package reasoner

// Context holds the saturation state for a single concept.
type Context struct {
	id ConceptID

	// S(C): set of all derived superclasses. Maps ConceptID → struct{}.
	superSet map[ConceptID]struct{}

	// Forward links: linkMap[r] = list of concepts D such that (C, D) ∈ R(r).
	linkMap [][]ConceptID

	// Reverse links: predMap[r] = list of concepts E such that (E, C) ∈ R(r).
	predMap [][]ConceptID
}

// workItem represents a pending inference to process.
type workItem struct {
	concept ConceptID
	added   ConceptID
}

// linkItem represents a newly added role link to process.
type linkItem struct {
	source ConceptID
	role   RoleID
	target ConceptID
}

// Saturate runs the single-threaded EL saturation algorithm.
// It applies completion rules CR1–CR5, CR10, CR11 until no new inferences can be derived.
func Saturate(st *SymbolTable, store *AxiomStore) []Context {
	n := st.ConceptCount()
	nr := st.RoleCount()

	contexts := make([]Context, n)
	for c := ConceptID(0); c < ConceptID(n); c++ {
		contexts[c].id = c
		contexts[c].superSet = make(map[ConceptID]struct{}, 8)
		contexts[c].linkMap = make([][]ConceptID, nr)
		contexts[c].predMap = make([][]ConceptID, nr)
	}

	// Worklist for concept subsumption propagation (CR1, CR2, CR3).
	worklist := make([]workItem, 0, n*2)

	// Link worklist for link-triggered rules (CR4, CR5, CR10, CR11).
	linkWorklist := make([]linkItem, 0, n)

	// Initialize: S(C) = {C, Top} for each named concept.
	for c := ConceptID(0); c < ConceptID(n); c++ {
		contexts[c].superSet[c] = struct{}{}
		contexts[c].superSet[Top] = struct{}{}
		worklist = append(worklist, workItem{c, c})
		worklist = append(worklist, workItem{c, Top})
	}

	// Main saturation loop.
	for len(worklist) > 0 || len(linkWorklist) > 0 {
		// Process concept worklist items first (LIFO for cache locality).
		for len(worklist) > 0 {
			item := worklist[len(worklist)-1]
			worklist = worklist[:len(worklist)-1]

			c := item.concept
			d := item.added // D was just added to S(C)

			// CR1: If D ∈ S(C) and D ⊑ E in store, add E to S(C).
			if int(d) < len(store.subToSups) {
				for _, e := range store.subToSups[d] {
					if _, exists := contexts[c].superSet[e]; !exists {
						contexts[c].superSet[e] = struct{}{}
						worklist = append(worklist, workItem{c, e})
					}
				}
			}

			// CR2: For each (D, D') or (D', D) conjunction axiom where D' ∈ S(C).
			if int(d) < len(store.conjIndex) && store.conjIndex[d] != nil {
				for d2, results := range store.conjIndex[d] {
					if _, exists := contexts[c].superSet[d2]; exists {
						for _, e := range results {
							if _, exists2 := contexts[c].superSet[e]; !exists2 {
								contexts[c].superSet[e] = struct{}{}
								worklist = append(worklist, workItem{c, e})
							}
						}
					}
				}
			}

			// CR3: If D ⊑ ∃R.B, add link (C, B) to R(R).
			if int(d) < len(store.existRight) {
				for _, rf := range store.existRight[d] {
					if addLink(&contexts[c], &contexts[rf.Fill], rf.Role) {
						linkWorklist = append(linkWorklist, linkItem{c, rf.Role, rf.Fill})
					}
				}
			}

			// CR4 backward: D was added to S(C). For each predecessor E
			// that has a link (E, C) via role R, check if ∃R.D ⊑ F.
			for r := RoleID(0); r < RoleID(nr); r++ {
				for _, pred := range contexts[c].predMap[r] {
					if int(r) < len(store.existLeft) && store.existLeft[r] != nil {
						if sups, ok := store.existLeft[r][d]; ok {
							for _, f := range sups {
								if _, exists := contexts[pred].superSet[f]; !exists {
									contexts[pred].superSet[f] = struct{}{}
									worklist = append(worklist, workItem{pred, f})
								}
							}
						}
					}
				}
			}
		}

		// Process link worklist items.
		for len(linkWorklist) > 0 {
			li := linkWorklist[len(linkWorklist)-1]
			linkWorklist = linkWorklist[:len(linkWorklist)-1]

			c := li.source
			r := li.role
			d := li.target

			// CR4 forward: (C, D) ∈ R(R). For each E in S(D), check ∃R.E ⊑ F.
			if int(r) < len(store.existLeft) && store.existLeft[r] != nil {
				for e := range contexts[d].superSet {
					if sups, ok := store.existLeft[r][e]; ok {
						for _, f := range sups {
							if _, exists := contexts[c].superSet[f]; !exists {
								contexts[c].superSet[f] = struct{}{}
								worklist = append(worklist, workItem{c, f})
							}
						}
					}
				}
			}

			// CR5: If ⊥ ∈ S(D), add ⊥ to S(C).
			if _, hasBottom := contexts[d].superSet[Bottom]; hasBottom {
				if _, exists := contexts[c].superSet[Bottom]; !exists {
					contexts[c].superSet[Bottom] = struct{}{}
					worklist = append(worklist, workItem{c, Bottom})
				}
			}

			// CR10: Role subsumption. If R ⊑ S, add (C, D) to R(S).
			if int(r) < len(store.roleSubs) {
				for _, s := range store.roleSubs[r] {
					if addLink(&contexts[c], &contexts[d], s) {
						linkWorklist = append(linkWorklist, linkItem{c, s, d})
					}
				}
			}

			// CR11: Role composition. If (E, C) ∈ R(R1) and R1 ∘ R ⊑ S, add (E, D) to R(S).
			for r1 := RoleID(0); r1 < RoleID(nr); r1++ {
				if int(r1) < len(store.roleChains) && store.roleChains[r1] != nil {
					if chains, ok := store.roleChains[r1][r]; ok {
						for _, pred := range contexts[c].predMap[r1] {
							for _, s := range chains {
								if addLink(&contexts[pred], &contexts[d], s) {
									linkWorklist = append(linkWorklist, linkItem{pred, s, d})
								}
							}
						}
					}
				}
			}

			// CR11 (second half): If (C, D) ∈ R(R) and (D, E) ∈ R(R2) and R ∘ R2 ⊑ S.
			if int(r) < len(store.roleChains) && store.roleChains[r] != nil {
				for r2, chains := range store.roleChains[r] {
					for _, e := range contexts[d].linkMap[r2] {
						for _, s := range chains {
							if addLink(&contexts[c], &contexts[e], s) {
								linkWorklist = append(linkWorklist, linkItem{c, s, e})
							}
						}
					}
				}
			}
		}
	}

	return contexts
}

// addLink adds (source, target) to R(role), updating both forward and reverse indices.
// Returns true if the link was new.
func addLink(source, target *Context, role RoleID) bool {
	// Check if link already exists.
	for _, existing := range source.linkMap[role] {
		if existing == target.id {
			return false
		}
	}
	source.linkMap[role] = append(source.linkMap[role], target.id)
	target.predMap[role] = append(target.predMap[role], source.id)
	return true
}
