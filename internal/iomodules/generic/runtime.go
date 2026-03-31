package generic

// Start input module reader in background
func (mod *InModule) Start() (err error) {
	mod.wg.Add(1)
	go mod.read()
	return
}

// Gracefully shutdown input module
func (mod *InModule) Shutdown() (err error) {
	if mod == nil {
		return
	}

	if mod.cancel != nil {
		mod.cancel()
	}

	if mod.sink != nil {
		err = mod.sink.Close()
		if err != nil {
			return
		}
	}

	mod.wg.Wait()
	return
}

// Gracefully shutdown output module
func (mod *OutModule) Shutdown() (err error) {
	if mod == nil {
		return
	}

	_, err = mod.FlushBuffer()
	if err != nil {
		return
	}

	err = mod.sink.Close()
	if err != nil {
		return
	}
	return
}
