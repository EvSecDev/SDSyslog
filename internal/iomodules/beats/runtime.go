package beats

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
