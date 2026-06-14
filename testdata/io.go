package testdata

import (
	"net"
	"os"
)

func ReadFile() {
	os.ReadFile("test.txt")
}

func DialNetwork() {
	net.Dial("tcp", "localhost:8080")
}
