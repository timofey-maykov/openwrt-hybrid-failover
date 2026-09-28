package routers

// SetSeams swaps the UCI lookup and key-file check for tests.
func SetSeams(uciGet func(string) (string, bool), exists func(string) bool) func() {
	origGet, origExists := localUCIGet, fileExists
	localUCIGet, fileExists = uciGet, exists
	return func() { localUCIGet, fileExists = origGet, origExists }
}
