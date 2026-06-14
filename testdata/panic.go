package testdata

func PanicDirect() {
	panic("oh no")
}

func PanicViaHelper() {
	helper := func() {
		panic("from helper")
	}
	helper()
}
