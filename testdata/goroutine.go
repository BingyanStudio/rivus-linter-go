package testdata

func LaunchGoroutine() {
	go func() {
		// async work
	}()
}

func LaunchNamed() {
	go helper()
}

func helper() {}
