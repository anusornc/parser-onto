package ontology

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
)

const writerBufferSize = 256 * 1024 // 256 KB

// WriteJSON writes the ontology as JSON to the given writer.
func WriteJSON(ont *Ontology, w io.Writer) error {
	bw := bufio.NewWriterSize(w, writerBufferSize)
	enc := json.NewEncoder(bw)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(ont); err != nil {
		return err
	}
	return bw.Flush()
}

// WriteJSONFile writes the ontology as JSON to the given file path.
func WriteJSONFile(ont *Ontology, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return WriteJSON(ont, f)
}

// WriteJSONPretty writes indented JSON to the given writer.
func WriteJSONPretty(ont *Ontology, w io.Writer) error {
	bw := bufio.NewWriterSize(w, writerBufferSize)
	enc := json.NewEncoder(bw)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(ont); err != nil {
		return err
	}
	return bw.Flush()
}
