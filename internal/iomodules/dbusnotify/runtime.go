package dbusnotify

// Gracefully stops module
func (mod *OutModule) Shutdown() (err error) {
	if mod == nil {
		return
	}

	if mod.conn != nil {
		err = mod.conn.Close()
	}
	return
}
