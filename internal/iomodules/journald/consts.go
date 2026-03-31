package journald

import "sdsyslog/pkg/protocol"

const (
	DefaultURL            string = "http://localhost:19532"
	FieldTruncationSuffix string = "[...TRUNCATED]"
	MaxTruncatedFieldLen  int    = protocol.MaxCtxValLen - len(FieldTruncationSuffix)
)
