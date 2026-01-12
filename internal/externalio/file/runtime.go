package file

// Gracefully stops module
func (mod *InModule) Shutdown() (err error) {
	if mod == nil {
		return
	}
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
