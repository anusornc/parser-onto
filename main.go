package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nodeadmin/chebi-parser/ontology"
)

func main() {
	input := flag.String("input", "", "Path to ChEBI ontology file (.obo or .owl)")
	output := flag.String("output", "", "Path to output JSON file (default: stdout)")
	format := flag.String("format", "auto", "Input format: auto, obo, owl")
	pretty := flag.Bool("pretty", false, "Pretty-print JSON output")
	flag.Parse()

	if *input == "" {
		fmt.Fprintln(os.Stderr, "Usage: chebi-parser -input <file> [-output <file>] [-format auto|obo|owl] [-pretty]")
		os.Exit(1)
	}

	// Detect format
	inputFmt := detectFormat(*input, *format)
	if inputFmt == "" {
		fmt.Fprintf(os.Stderr, "Error: cannot detect format for %q. Use -format obo or -format owl.\n", *input)
		os.Exit(1)
	}

	// Open input
	f, err := os.Open(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening input: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	// Parse
	fmt.Fprintf(os.Stderr, "Parsing %s as %s...\n", filepath.Base(*input), inputFmt)
	start := time.Now()

	var ont *ontology.Ontology
	switch inputFmt {
	case "obo":
		ont, err = ontology.ParseOBO(f)
	case "owl":
		ont, err = ontology.ParseOWL(f)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing: %v\n", err)
		os.Exit(1)
	}

	elapsed := time.Since(start)
	fmt.Fprintf(os.Stderr, "Parsed %d terms in %v\n", len(ont.Terms), elapsed)

	// Write output
	start = time.Now()
	if *output != "" {
		if *pretty {
			outFile, err := os.Create(*output)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating output: %v\n", err)
				os.Exit(1)
			}
			defer outFile.Close()
			err = ontology.WriteJSONPretty(ont, outFile)
		} else {
			err = ontology.WriteJSONFile(ont, *output)
		}
	} else {
		if *pretty {
			err = ontology.WriteJSONPretty(ont, os.Stdout)
		} else {
			err = ontology.WriteJSON(ont, os.Stdout)
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
		os.Exit(1)
	}

	if *output != "" {
		writeElapsed := time.Since(start)
		fmt.Fprintf(os.Stderr, "Wrote JSON in %v\n", writeElapsed)
	}
}

func detectFormat(path, explicit string) string {
	if explicit != "auto" {
		return explicit
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".obo":
		return "obo"
	case ".owl", ".xml", ".rdf":
		return "owl"
	}
	return ""
}
