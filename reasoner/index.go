package reasoner

// ConceptID is an integer identifier for a named concept.
type ConceptID uint32

// RoleID is an integer identifier for an object property (role).
type RoleID uint32

const (
	Top    ConceptID = 0 // owl:Thing
	Bottom ConceptID = 1 // owl:Nothing
)

// SymbolTable maps string IRIs/names to integer IDs for the reasoner's inner loop.
type SymbolTable struct {
	conceptToID map[string]ConceptID
	idToConcept []string
	roleToID    map[string]RoleID
	idToRole    []string
}

func NewSymbolTable() *SymbolTable {
	concepts := make([]string, 2, 250000)
	concepts[Top] = "owl:Thing"
	concepts[Bottom] = "owl:Nothing"

	st := &SymbolTable{
		conceptToID: make(map[string]ConceptID, 250000),
		idToConcept: concepts,
		roleToID:    make(map[string]RoleID, 32),
		idToRole:    make([]string, 0, 32),
	}
	st.conceptToID["owl:Thing"] = Top
	st.conceptToID["owl:Nothing"] = Bottom
	return st
}

// InternConcept returns the ConceptID for the given name, creating one if needed.
func (st *SymbolTable) InternConcept(name string) ConceptID {
	if id, ok := st.conceptToID[name]; ok {
		return id
	}
	id := ConceptID(len(st.idToConcept))
	st.conceptToID[name] = id
	st.idToConcept = append(st.idToConcept, name)
	return id
}

// InternRole returns the RoleID for the given name, creating one if needed.
func (st *SymbolTable) InternRole(name string) RoleID {
	if id, ok := st.roleToID[name]; ok {
		return id
	}
	id := RoleID(len(st.idToRole))
	st.roleToID[name] = id
	st.idToRole = append(st.idToRole, name)
	return id
}

func (st *SymbolTable) ConceptCount() int { return len(st.idToConcept) }
func (st *SymbolTable) RoleCount() int    { return len(st.idToRole) }

// ConceptName returns the string name for a ConceptID.
func (st *SymbolTable) ConceptName(id ConceptID) string {
	if int(id) < len(st.idToConcept) {
		return st.idToConcept[id]
	}
	return ""
}

// RoleName returns the string name for a RoleID.
func (st *SymbolTable) RoleName(id RoleID) string {
	if int(id) < len(st.idToRole) {
		return st.idToRole[id]
	}
	return ""
}

// FreshConcept creates a new anonymous concept with a generated name.
func (st *SymbolTable) FreshConcept() ConceptID {
	id := ConceptID(len(st.idToConcept))
	st.idToConcept = append(st.idToConcept, "")
	return id
}
