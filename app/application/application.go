package main

import (
	"encoding/json"
	"fmt"
	"io"
)

//////////////////// ApplicationHandler implementation:
type ConnInfo struct {
	someshit string
}

func (ci *ConnInfo) HandlePrint(message interface{}) {
	foo, ok := message.(*Message)
	if ok {
		fmt.Println("conninfo: ", ci.someshit, "\ninfo1:", foo.Userinfo1, "\ninfo1:", foo.Userinfo2)
	}
}
func (ci *ConnInfo) NewMessage() interface{} {
	return &Message{}
}

type Message struct {
	Userinfo1 string
	Userinfo2 string
}

//////////////////////////////////////////////////
type ApplicationHandler interface {
	HandlePrint(message interface{})
	NewMessage() interface{}
}

type Opcode byte

const (
	OpPrint Opcode = 1
)

type Connection struct {
	r          io.Reader
	apphandler ApplicationHandler
}

type messageWithMeta struct {
	Opcode  Opcode
	Message interface{}
}

func Handle(conn *Connection) {
	mm := messageWithMeta{Message: conn.apphandler.NewMessage()}
	json.NewDecoder(conn.r).Decode(&mm)
	switch mm.Opcode {
	case OpPrint:
		conn.apphandler.HandlePrint(mm.Message)
	default:
		println("unknown opcode ", mm.Opcode)
	}
}

func main() {
	var r reader
	Handle(&Connection{r: &r, apphandler: &ConnInfo{someshit: "someconninfo"}})
}

//////////////////////////// io.Reader implementation:
type reader []byte

func (c *reader) Read(b []byte) (int, error) {
	bb, err := json.Marshal(messageWithMeta{Opcode: OpPrint, Message: Message{Userinfo1: "firstinfo", Userinfo2: "secondinfo"}})
	if err != nil {
		println("this")
		return 0, err
	}
	copy(b, bb)
	//b = append(b[0:], bb...)
	return len(bb), nil
}
