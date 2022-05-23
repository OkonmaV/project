package encode

type LogsFlushLevel byte

const (
	ZeroLevel    LogsFlushLevel = 0 // none
	DebugLevel   LogsFlushLevel = 1 // all
	InfoLevel    LogsFlushLevel = 2 // infos, warnings, errors
	WarningLevel LogsFlushLevel = 3 // warnings, errors
	ErrorLevel   LogsFlushLevel = 4 // errors
)
