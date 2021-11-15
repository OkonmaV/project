package defaultlogger

type DefaultLogger interface {
	Debug(string, string)
	Info(string, string)
	Warning(string, string)
	Error(string, error)
}

type Defaultlogger struct{}

func (l *Defaultlogger) Debug(msg, info string) {
	println("["+msg+"]", "["+info+"]")
}

func (l *Defaultlogger) Info(msg, info string) {
	println("["+msg+"]", "["+info+"]")
}

func (l *Defaultlogger) Warning(msg, info string) {
	println("["+msg+"]", "["+info+"]")
}

func (l *Defaultlogger) Error(msg string, err error) {
	println("["+msg+"]", "["+err.Error()+"]")
}
