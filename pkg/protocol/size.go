package protocol

// Gets total size in memory for the payload (to include stable internal-go overheads)
func (payload Payload) Size() (bytes int) {
	bytes = len(payload.LogText) +
		len(payload.ApplicationName) + // string backing bytes
		len(payload.Hostname) + // string backing bytes
		len(payload.Facility) + // string backing bytes
		len(payload.Severity) + // string backing bytes
		8 + // HostID (int)
		8 + // LogID (int)
		8 + // MessageSeq (int)
		8 + // MessageSeqMax (int)
		24 + // Timestamp (time.Time)
		8 + // ProcessID (int)
		16 + // Facility string header
		16 + // Severity string header
		16 + // Hostname string header
		16 + // ApplicationName string header
		24 + // LogText []byte header
		8 // PaddingLen (int)
	return
}
