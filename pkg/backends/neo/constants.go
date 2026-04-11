package neo

// Shared OTel span event attribute keys used across the neo package.
// Extracted to constants to satisfy SonarQube rule S1192.
const (
	attrLogSeverity = "log.severity"
	attrLogQuery    = "log.query"
	attrLogMessage  = "log.message"
	attrSevDebug    = "debug"
	attrSevError    = "error"

	// Shared error messages used across multiple files.
	errFailedRunQuery    = "Failed to run query"
	errFailedRunQueryFmt = "Failed to run query: %s"
)
