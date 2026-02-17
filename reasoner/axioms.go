package reasoner

// RoleFiller pairs a role with its filler concept.
type RoleFiller struct {
	Role RoleID
	Fill ConceptID
}

// AxiomStore holds normalized axioms indexed for efficient lookup by the saturation rules.
//
// The six normal forms are:
//   NF1: A ⊑ B            (atomic subsumption)
//   NF2: A₁ ⊓ A₂ ⊑ B     (conjunction on the left)
//   NF3: A ⊑ ∃R.B         (existential on the right)
//   NF4: ∃R.A ⊑ B         (existential on the left)
//   NF5: R ⊑ S            (role subsumption)
//   NF6: R₁ ∘ R₂ ⊑ S     (role composition / property chain)
type AxiomStore struct {
	// NF1: subToSups[A] = list of B where A ⊑ B. Triggers CR1.
	subToSups [][]ConceptID

	// NF2: conjIndex[A1][A2] = list of B where A1 ⊓ A2 ⊑ B. Triggers CR2.
	// Stored symmetrically: both conjIndex[A1][A2] and conjIndex[A2][A1] contain B.
	conjIndex []map[ConceptID][]ConceptID

	// NF3: existRight[A] = list of (R, B) where A ⊑ ∃R.B. Triggers CR3.
	existRight [][]RoleFiller

	// NF4: existLeft[R][A] = list of B where ∃R.A ⊑ B. Triggers CR4.
	existLeft []map[ConceptID][]ConceptID

	// NF5: roleSubs[R] = list of S where R ⊑ S. Triggers CR10.
	roleSubs [][]RoleID

	// NF6: roleChains[R1][R2] = list of S where R1 ∘ R2 ⊑ S. Triggers CR11.
	roleChains []map[RoleID][]RoleID

	// Role properties.
	transitive []bool
	reflexive  []bool
}

// NewAxiomStore allocates an AxiomStore sized for the given symbol table.
func NewAxiomStore(st *SymbolTable) *AxiomStore {
	nc := st.ConceptCount()
	nr := st.RoleCount()

	s := &AxiomStore{
		subToSups:  make([][]ConceptID, nc),
		conjIndex:  make([]map[ConceptID][]ConceptID, nc),
		existRight: make([][]RoleFiller, nc),
		existLeft:  make([]map[ConceptID][]ConceptID, nr),
		roleSubs:   make([][]RoleID, nr),
		roleChains: make([]map[RoleID][]RoleID, nr),
		transitive: make([]bool, nr),
		reflexive:  make([]bool, nr),
	}
	return s
}

// Grow expands all concept-indexed slices to accommodate new concepts (e.g. fresh concepts).
func (s *AxiomStore) Grow(nc int) {
	for len(s.subToSups) < nc {
		s.subToSups = append(s.subToSups, nil)
	}
	for len(s.conjIndex) < nc {
		s.conjIndex = append(s.conjIndex, nil)
	}
	for len(s.existRight) < nc {
		s.existRight = append(s.existRight, nil)
	}
}

// GrowRoles expands all role-indexed slices.
func (s *AxiomStore) GrowRoles(nr int) {
	for len(s.existLeft) < nr {
		s.existLeft = append(s.existLeft, nil)
	}
	for len(s.roleSubs) < nr {
		s.roleSubs = append(s.roleSubs, nil)
	}
	for len(s.roleChains) < nr {
		s.roleChains = append(s.roleChains, nil)
	}
	for len(s.transitive) < nr {
		s.transitive = append(s.transitive, false)
	}
	for len(s.reflexive) < nr {
		s.reflexive = append(s.reflexive, false)
	}
}

// AddSubsumption adds NF1: sub ⊑ sup.
func (s *AxiomStore) AddSubsumption(sub, sup ConceptID) {
	s.subToSups[sub] = append(s.subToSups[sub], sup)
}

// AddConjunction adds NF2: left1 ⊓ left2 ⊑ right (stored symmetrically).
func (s *AxiomStore) AddConjunction(left1, left2, right ConceptID) {
	if s.conjIndex[left1] == nil {
		s.conjIndex[left1] = make(map[ConceptID][]ConceptID, 4)
	}
	s.conjIndex[left1][left2] = append(s.conjIndex[left1][left2], right)

	if left1 != left2 {
		if s.conjIndex[left2] == nil {
			s.conjIndex[left2] = make(map[ConceptID][]ConceptID, 4)
		}
		s.conjIndex[left2][left1] = append(s.conjIndex[left2][left1], right)
	}
}

// AddExistRight adds NF3: sub ⊑ ∃role.fill.
func (s *AxiomStore) AddExistRight(sub ConceptID, role RoleID, fill ConceptID) {
	s.existRight[sub] = append(s.existRight[sub], RoleFiller{Role: role, Fill: fill})
}

// AddExistLeft adds NF4: ∃role.fill ⊑ sup.
func (s *AxiomStore) AddExistLeft(role RoleID, fill ConceptID, sup ConceptID) {
	if s.existLeft[role] == nil {
		s.existLeft[role] = make(map[ConceptID][]ConceptID, 4)
	}
	s.existLeft[role][fill] = append(s.existLeft[role][fill], sup)
}

// AddRoleSub adds NF5: sub ⊑ sup.
func (s *AxiomStore) AddRoleSub(sub, sup RoleID) {
	s.roleSubs[sub] = append(s.roleSubs[sub], sup)
}

// AddRoleChain adds NF6: left1 ∘ left2 ⊑ right.
func (s *AxiomStore) AddRoleChain(left1, left2, right RoleID) {
	if s.roleChains[left1] == nil {
		s.roleChains[left1] = make(map[RoleID][]RoleID, 4)
	}
	s.roleChains[left1][left2] = append(s.roleChains[left1][left2], right)
}

// SetTransitive marks a role as transitive (equivalent to R ∘ R ⊑ R).
func (s *AxiomStore) SetTransitive(r RoleID) {
	s.transitive[r] = true
	s.AddRoleChain(r, r, r)
}

// SetReflexive marks a role as reflexive.
func (s *AxiomStore) SetReflexive(r RoleID) {
	s.reflexive[r] = true
}

// IsTransitive returns whether role r is transitive.
func (s *AxiomStore) IsTransitive(r RoleID) bool {
	return int(r) < len(s.transitive) && s.transitive[r]
}
