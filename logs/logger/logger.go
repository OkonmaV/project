package logger

type Logger interface {
	LogsWriter
	NewSubLogger(tags ...string) Logger
	NewPackageSubLogger(logsBufLen int, tags ...string) PackageLogger
}

type PackageLogger interface {
	LogsWriter
	Flush()
}

type LogsWriter interface {
	Debug(string, string)
	Info(string, string)
	Warning(string, string)
	Error(string, error)
}

type LogsFlusher interface {
	NewLogsContainer(tags ...string) Logger
	Close()
	Done()
	DoneWithTimeout()
}
