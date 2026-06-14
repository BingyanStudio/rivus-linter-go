package model

// FlagType represents a side-effect category.
type FlagType uint16

const (
	FlagPanic      FlagType = 1 << iota // P
	FlagGoroutine                       // G
	FlagContext                         // D
	FlagIO                             // I
	FlagSideEffect                     // S
	FlagUnsafe                         // U
	FlagTime                           // T
	FlagExit                           // X
	FlagReflect                        // R
	FlagCGO                            // C
)

// flagNames maps each FlagType to its single-letter name.
var flagNames = map[FlagType]string{
	FlagPanic:      "P",
	FlagGoroutine:  "G",
	FlagContext:    "D",
	FlagIO:        "I",
	FlagSideEffect: "S",
	FlagUnsafe:    "U",
	FlagTime:      "T",
	FlagExit:      "X",
	FlagReflect:   "R",
	FlagCGO:       "C",
}

// String returns the single-letter name for a FlagType.
func (f FlagType) String() string {
	if name, ok := flagNames[f]; ok {
		return name
	}
	return "?"
}

// FlagSet is a bitmask of FlagType values.
type FlagSet uint16

// Has reports whether the set contains the given flag.
func (s FlagSet) Has(f FlagType) bool {
	return s&FlagSet(f) != 0
}

// Add adds a flag to the set and returns the new set.
func (s FlagSet) Add(f FlagType) FlagSet {
	return s | FlagSet(f)
}

// Union returns the union of two flag sets.
func (s FlagSet) Union(other FlagSet) FlagSet {
	return s | other
}

// IsEmpty reports whether the set has no flags.
func (s FlagSet) IsEmpty() bool {
	return s == 0
}

// String returns a comma-separated list of flag names.
func (s FlagSet) String() string {
	if s == 0 {
		return ""
	}
	var names []string
	for f, name := range flagNames {
		if s.Has(f) {
			names = append(names, name)
		}
	}
	// Simple sort by name for deterministic output.
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	result := ""
	for i, n := range names {
		if i > 0 {
			result += ", "
		}
		result += n
	}
	return result
}

// AllFlags returns a slice of all defined FlagType values.
func AllFlags() []FlagType {
	return []FlagType{
		FlagPanic, FlagGoroutine, FlagContext, FlagIO, FlagSideEffect,
		FlagUnsafe, FlagTime, FlagExit, FlagReflect, FlagCGO,
	}
}
