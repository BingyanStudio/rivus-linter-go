package model

import (
	"go/token"
	"path/filepath"
)

// Position represents a source code location.
type Position struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Col  int    `json:"col"`
}

// PositionFromToken converts a token.Pos to a Position using the provided FileSet.
// If pos is invalid, returns a zero Position.
func PositionFromToken(fset *token.FileSet, pos token.Pos) Position {
	if !pos.IsValid() {
		return Position{}
	}
	p := fset.Position(pos)
	return Position{
		File: filepath.Base(p.Filename),
		Line: p.Line,
		Col:  p.Column,
	}
}
