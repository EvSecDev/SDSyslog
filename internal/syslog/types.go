package syslog

type LogFacility struct {
	FacilityToCode map[string]uint16
	CodeToFacility map[uint16]string
}

type LogSeverity struct {
	SeverityToCode map[string]uint16
	CodeToSeverity map[uint16]string
}
