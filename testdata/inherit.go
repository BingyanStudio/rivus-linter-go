package testdata

import "os"

// doIO performs I/O — flagged as {I}.
func doIO() {
	os.ReadFile("test.txt")
}

// CallIO calls doIO — inherits {I} from doIO.
func CallIO() {
	doIO()
}

// doSideEffect reads a global — flagged as {S}.
var myGlobal int

func doSideEffect() {
	_ = myGlobal
}

// CallSideEffect calls doSideEffect — inherits {S}.
func CallSideEffect() {
	doSideEffect()
}
