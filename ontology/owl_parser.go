package ontology

import (
	"encoding/xml"
	"io"
	"strings"
)

// OWL/RDF namespace URIs
const (
	nsOWL  = "http://www.w3.org/2002/07/owl#"
	nsRDF  = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	nsRDFS = "http://www.w3.org/2000/01/rdf-schema#"
	nsOBO  = "http://purl.obolibrary.org/obo/"
)

// ParseOWL parses a ChEBI OWL/RDF-XML ontology from the given reader.
func ParseOWL(r io.Reader) (*Ontology, error) {
	decoder := xml.NewDecoder(r)
	pool := newInternPool()

	ont := &Ontology{
		Terms: make([]Term, 0, initialTermCapacity),
	}

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}

		switch {
		case matchElement(se, nsOWL, "Class"):
			term := parseOWLClass(decoder, se, pool)
			if term.ID != "" {
				ont.Terms = append(ont.Terms, term)
			}
		case matchElement(se, nsOWL, "Ontology"):
			parseOWLOntologyHeader(decoder, se, ont)
		case matchElement(se, nsOWL, "ObjectProperty"):
			td := parseOWLObjectProperty(decoder, se, pool)
			if td.ID != "" {
				ont.TypeDefs = append(ont.TypeDefs, td)
			}
		case matchElement(se, nsRDF, "RDF"):
			// Container element — descend into it, don't skip
		default:
			decoder.Skip()
		}
	}

	return ont, nil
}

func matchElement(se xml.StartElement, ns, local string) bool {
	return se.Name.Space == ns && se.Name.Local == local
}

func getAttr(se xml.StartElement, ns, local string) string {
	for _, a := range se.Attr {
		if a.Name.Space == ns && a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

func oboIDFromURI(uri string) string {
	// Convert http://purl.obolibrary.org/obo/CHEBI_12345 to CHEBI:12345
	if strings.HasPrefix(uri, nsOBO) {
		id := uri[len(nsOBO):]
		if idx := strings.IndexByte(id, '_'); idx >= 0 {
			return id[:idx] + ":" + id[idx+1:]
		}
		return id
	}
	return uri
}

func parseOWLOntologyHeader(decoder *xml.Decoder, se xml.StartElement, ont *Ontology) {
	about := getAttr(se, nsRDF, "about")
	if about != "" {
		ont.Ontology = about
	}

	for {
		tok, err := decoder.Token()
		if err != nil {
			return
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "versionIRI" {
				v := getAttr(t, nsRDF, "resource")
				if v != "" {
					ont.DataVersion = v
				}
			}
			decoder.Skip()
		case xml.EndElement:
			return
		}
	}
}

func parseOWLClass(decoder *xml.Decoder, se xml.StartElement, pool *internPool) Term {
	var t Term

	about := getAttr(se, nsRDF, "about")
	if about != "" {
		t.ID = oboIDFromURI(about)
	}

	for {
		tok, err := decoder.Token()
		if err != nil {
			return t
		}

		switch el := tok.(type) {
		case xml.StartElement:
			switch {
			case matchElement(el, nsRDFS, "label"):
				t.Name = readCharData(decoder)
			case matchElement(el, nsRDFS, "subClassOf"):
				res := getAttr(el, nsRDF, "resource")
				if res != "" {
					t.Relationships = append(t.Relationships, Relationship{
						Type:     pool.get("is_a"),
						TargetID: oboIDFromURI(res),
					})
					decoder.Skip()
				} else {
					// Complex restriction — parse owl:Restriction
					rel := parseOWLRestriction(decoder, pool)
					if rel.Type != "" && rel.TargetID != "" {
						t.Relationships = append(t.Relationships, rel)
					}
				}
			case el.Name.Local == "deprecated":
				val := readCharData(decoder)
				t.IsObsolete = val == "true"
			case el.Name.Local == "hasAlternativeId":
				t.AltIDs = append(t.AltIDs, readCharData(decoder))
			case el.Name.Local == "Definition" || el.Name.Local == "definition":
				t.Definition = readCharData(decoder)
			case el.Name.Local == "hasExactSynonym":
				t.Synonyms = append(t.Synonyms, Synonym{
					Text:  readCharData(decoder),
					Scope: "EXACT",
				})
			case el.Name.Local == "hasBroadSynonym":
				t.Synonyms = append(t.Synonyms, Synonym{
					Text:  readCharData(decoder),
					Scope: "BROAD",
				})
			case el.Name.Local == "hasNarrowSynonym":
				t.Synonyms = append(t.Synonyms, Synonym{
					Text:  readCharData(decoder),
					Scope: "NARROW",
				})
			case el.Name.Local == "hasRelatedSynonym":
				t.Synonyms = append(t.Synonyms, Synonym{
					Text:  readCharData(decoder),
					Scope: "RELATED",
				})
			case el.Name.Local == "hasDbXref" || el.Name.Local == "hasDbXRef":
				t.Xrefs = append(t.Xrefs, readCharData(decoder))
			case el.Name.Local == "inSubset":
				res := getAttr(el, nsRDF, "resource")
				if res != "" {
					t.Subsets = append(t.Subsets, pool.get(oboIDFromURI(res)))
				}
				decoder.Skip()
			case el.Name.Local == "comment":
				t.Comment = readCharData(decoder)
			default:
				// Capture as property if it has text content
				name := el.Name.Local
				val := readCharData(decoder)
				if val != "" {
					if t.Properties == nil {
						t.Properties = make(map[string]string, 4)
					}
					t.Properties[name] = val
				}
			}
		case xml.EndElement:
			// End of owl:Class
			return t
		}
	}
}

// parseOWLRestriction parses the content inside a rdfs:subClassOf that contains
// an owl:Restriction with onProperty and someValuesFrom.
func parseOWLRestriction(decoder *xml.Decoder, pool *internPool) Relationship {
	var rel Relationship
	depth := 0
	for {
		tok, err := decoder.Token()
		if err != nil {
			return rel
		}
		switch el := tok.(type) {
		case xml.StartElement:
			depth++
			switch {
			case matchElement(el, nsOWL, "onProperty"):
				res := getAttr(el, nsRDF, "resource")
				if res != "" {
					rel.Type = pool.get(oboIDFromURI(res))
				}
				decoder.Skip()
				depth--
			case matchElement(el, nsOWL, "someValuesFrom"):
				res := getAttr(el, nsRDF, "resource")
				if res != "" {
					rel.TargetID = oboIDFromURI(res)
				}
				decoder.Skip()
				depth--
			default:
				decoder.Skip()
				depth--
			}
		case xml.EndElement:
			depth--
			if depth < 0 {
				return rel
			}
		}
	}
}

const nsOBOInOwl = "http://www.geneontology.org/formats/oboInOwl#"

// parseOWLObjectProperty parses an owl:ObjectProperty element.
func parseOWLObjectProperty(decoder *xml.Decoder, se xml.StartElement, pool *internPool) TypeDef {
	var td TypeDef
	about := getAttr(se, nsRDF, "about")
	if about != "" {
		td.ID = oboIDFromURI(about)
	}

	for {
		tok, err := decoder.Token()
		if err != nil {
			return td
		}
		switch el := tok.(type) {
		case xml.StartElement:
			switch {
			case matchElement(el, nsRDF, "type"):
				res := getAttr(el, nsRDF, "resource")
				if res == nsOWL+"TransitiveProperty" {
					td.IsTransitive = true
				} else if res == nsOWL+"ReflexiveProperty" {
					td.IsReflexive = true
				}
				decoder.Skip()
			case matchElement(el, nsRDFS, "label"):
				td.Name = readCharData(decoder)
			default:
				decoder.Skip()
			}
		case xml.EndElement:
			return td
		}
	}
}

func readCharData(decoder *xml.Decoder) string {
	var sb strings.Builder
	for {
		tok, err := decoder.Token()
		if err != nil {
			return sb.String()
		}
		switch t := tok.(type) {
		case xml.CharData:
			sb.Write(t)
		case xml.StartElement:
			// Nested element — recurse into it but still collect text
			inner := readCharData(decoder)
			if inner != "" {
				sb.WriteString(inner)
			}
		case xml.EndElement:
			return sb.String()
		}
	}
}
