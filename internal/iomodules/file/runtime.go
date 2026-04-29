package file

// Starts reader for file
func (mod *InModule) Start() (err error) {
	// Start reader in go routine
	mod.wg.Add(1)
	go mod.reader()
	return
}

// Gracefully stops module
func (mod *InModule) Shutdown() (err error) {
	if mod == nil {
		return
	}

	if mod.cancel != nil {
		mod.cancel()
	}

	mod.wg.Wait()

	if mod.sink != nil {
		err = mod.sink.Close()
	}
	return
}

// Gracefully stops module
func (mod *OutModule) Shutdown() (err error) {
	if mod == nil {
		return
	}
	if mod.sink != nil {
		err = mod.sink.Close()
	}
	return
}
