package reasoner

import (
	"runtime"
)

func SaturateParallel(st *SymbolTable, store *AxiomStore, workers int) []Context {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers == 1 {
		return Saturate(st, store)
	}
	return Saturate(st, store)
}
