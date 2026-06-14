package testdata

import (
	"math/rand"
	"os"
)

var globalVar int

func ReadGlobal() int {
	return globalVar
}

func WriteGlobal() {
	globalVar = 42
}

func ReadEnv() {
	os.Getenv("HOME")
}

func UseRandom() {
	rand.Intn(100)
}
