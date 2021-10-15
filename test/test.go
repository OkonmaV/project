package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"project/test/logscontainer"
	"project/test/logscontainer/flushers"
	"strconv"
	"strings"
	"sync"

	"time"

	"github.com/BurntSushi/toml"
	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
	"github.com/gobwas/ws"
)

type config struct {
	ConfiguratorAddr string
}

const thisservicename = "test.test"

type HttpService interface {
	Handle(r *suckhttp.Request, logger *logscontainer.WrappedLogsContainer) (*suckhttp.Response, error)
}

type HandlerFunc func(ctx context.Context, conn net.Conn) error

func InitNewService(l *logscontainer.LogsContainer, thisservicename, configuratoraddr string, resolveServices ...string) error {
	conf := &config{}
	if _, err := toml.DecodeFile("config.toml", conf); err != nil {
		return err
	}
	if conf.ConfiguratorAddr == "" {
		return errors.New("some fields in config.toml are empty or not specified")
	}

	ctx, cancel := CreateContextWithInterruptSignal()
	logsctx, logscancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		logscancel()
		l.WaitAllFlushesDone()
	}()

	l, err := logscontainer.NewLogsContainer(logsctx, flushers.NewConsoleFlusher(thisservicename), 1, time.Second, 1)
	if err != nil {
		return err
	}
	wl := l.Wrap(logscontainer.LogTags{1: "conf.conf"})

	_, _, err = ConnectToConfigurator(ctx, wl, configuratoraddr, thisservicename, nil)
	if err != nil {
		return err
	}
	return nil
}

func ConnectToConfigurator(ctx context.Context, l *logscontainer.WrappedLogsContainer, configuratoraddr string, thisservicename string, innerServices map[string]string) (net.Conn, string, error) {
	var addr string
	d := ws.Dialer{
		Header: ws.HandshakeHeaderHTTP(http.Header{
			"x-get-addr": []string{"1"},
		}),
		OnHeader: func(key, value []byte) (err error) {
			addr = string(value)
			return nil
		},
	}
	conn, _, _, err := d.Dial(context.Background(), suckutils.ConcatFour("ws://", configuratoraddr, "/", thisservicename))
	if err != nil {
		return nil, "", err
	}
	conn.RemoteAddr()
	c := &Configurator{conn: conn}

	go func() {
		for {
			select {
			case <-ctx.Done():

			}
		}
	}()

	go func() {
		handlews(ctx, l, c, configuratoraddr, thisservicename)
	}()
	return conn, addr, nil
}

type Configurator struct {
	conn          net.Conn
	innerservices map[string]net.Conn
	done          chan struct{}
}

func handlews(ctx context.Context, l *logscontainer.WrappedLogsContainer, c *Configurator, configuratoraddr string, thisservicename string) {
	reconnectconf := make(chan struct{}, 1)
	var err error
	for {
		select {
		case <-reconnectconf:
			l.Error("Reconnect", errors.New("lost conn, trying to reconnect"))
			for {
				c.conn, _, _, err = ws.Dial(context.Background(), suckutils.ConcatFour("ws://", configuratoraddr, "/", thisservicename))
				if err != nil {
					l.Warning("configurator", "unsuccessful reconnect")
					time.Sleep(time.Second * 2)
				}
				l.Info("Reconnect", "reconnected!")
				break
			}

		default:
			frame, err := ws.ReadFrame(c.conn)
			if err != nil {
				if err == net.ErrClosed {
					return
				}
				fmt.Println("ReadFrame", err)
				c.conn.Close() //
				return
			}
			if frame.Header.Masked {
				ws.Cipher(frame.Payload, frame.Header.Mask, 0)
			}
			if frame.Header.OpCode.IsReserved() {
				fmt.Println(ws.ErrProtocolOpCodeReserved)
				return
			}
			if frame.Header.OpCode.IsControl() {
				switch {
				case frame.Header.OpCode == ws.OpClose:
					//TODO:
					break
				case frame.Header.OpCode == ws.OpPing:
					// TODO:
					break
				case frame.Header.OpCode == ws.OpPong:
					// TODO:
					break
				default:
					fmt.Println("OpControl", errors.New("not a control frame"))
				}
			}
			fmt.Println("PAYLOAD:", string(frame.Payload))

			// как-то обрабатываем
		}
	}
}

type Addr []byte

func ParseIPv4withPort(addr string) Addr {
	foo := strings.Split(addr, ":")
	if len(foo) != 2 {
		return nil
	}
	fmt.Println(1, foo)
	address := make([]byte, 0, 6)
	address = append(address, net.ParseIP(foo[0]).To4()...)
	address = append(address, []byte{0, 0}...)
	fmt.Println(2, address)
	port, err := strconv.ParseUint(foo[1], 10, 16)
	fmt.Println(3, "err", err, "port", port, cap(address), len(address))
	binary.BigEndian.PutUint16(address[4:], uint16(port))
	fmt.Println(88, address)
	fmt.Println(4, []byte{address[0], address[1], address[2], address[3], address[4], address[5], 0})
	return address
}
func (address Addr) String() string {
	if len(address) < 6 {
		if len(address) == 4 {
			return net.IPv4(address[0], address[1], address[2], address[3]).String()
		}
		return ""
	}
	return suckutils.ConcatThree(net.IPv4(address[0], address[1], address[2], address[3]).String(), ":", strconv.Itoa(int(binary.BigEndian.Uint16(address[4:]))))
}

func main() {
	f := ParseIPv4withPort("127.6.6.1:25")
	fmt.Println(5, f)
	fmt.Println(6, f[:4].String(), cap(f))
	addrwithstatus := append(f, 255)
	fmt.Println(7, addrwithstatus, cap(addrwithstatus), []byte("/"))
	// conn, addrToListen, err := ConnectToConfigurator("127.0.0.1:8089", "test.test")
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	// // i := uint16(1000)
	// // b := make([]byte, 2)
	// // binary.BigEndian.PutUint16(b, i)
	// // fmt.Println(b)
	// fmt.Println("Connected", conn.LocalAddr(), ">", conn.RemoteAddr())
	// //fmt.Println(ws.WriteFrame(conn, ws.MaskFrame(ws.NewFrame(ws.OpText, true, []byte("hi")))))
	// time.Sleep(time.Second)
	// fmt.Println(ws.WriteFrame(conn, ws.MaskFrame(ws.NewFrame(ws.OpClose, true, []byte{3, 232}))))
	// //fmt.Println(ws.WriteFrame(conn, ws.MaskFrame(ws.NewCloseFrame([]byte{3, 232, 115, 111, 109, 101, 32, 114, 101, 97, 115, 111, 110}))))

	// //fmt.Println(ws.WriteFrame(conn, ws.MaskFrame(ws.NewCloseFrame([]byte{}))))
	// time.Sleep(time.Second * 2)
	// //fmt.Println(ws.WriteFrame(conn, ws.MaskFrame(ws.NewCloseFrame([]byte{}))))
	// //conn.Close()
	// net.Listen("tcp", addrToListen)
	// fmt.Println("listen to", addrToListen)
	// time.Sleep(time.Hour)
}

func ServeHTTPService(ctx context.Context, l *logscontainer.LogsContainer, serviceName string, network, address string, connectionAlive bool, maxConnections int, handler HttpService) error {
	return ServeServiceWithContext(ctx, l, network, address, connectionAlive, maxConnections, func(ctx context.Context, conn net.Conn) error {
		request, err := suckhttp.ReadRequest(ctx, conn, time.Minute)
		if err != nil {
			return err
		}
		if request.GetHeader("x-request-id") == "" {
			return errors.New("not set x-request-id")
		}
		wl := l.Wrap(logscontainer.LogTags{2: request.GetHeader("x-request-id"), 3: request.GetRemoteAddr()})
		//l.Debug(logsName, suckutils.ConcatFour("Readed from ", request.GetRemoteAddr(), " for ", request.Time.String()))
		response, err := handler.Handle(request, wl)
		if err != nil {
			l.Error("Handle", err)
			if response == nil {
				response = suckhttp.NewResponse(500, "Internal Server Error")
			}
			if writeErr := response.Write(conn, time.Minute); writeErr != nil {
				l.Error("Write response", writeErr)
			}
			return err
		}
		//logger.Debug("Service", "Writing response...")
		err = response.Write(conn, time.Minute)
		if err != nil {
			l.Error("Write response", err)
		} else {
			l.Debug("Responce handling", "Done")
		}
		return err
	})
}

func ServeServiceWithContext(ctx context.Context, logger *logscontainer.LogsContainer, network, address string, connectionAlive bool, maxconnections int, handler HandlerFunc) error {
	l, err := net.Listen(network, address)
	if err != nil {
		return err
	}
	listenerLocker := sync.Mutex{}
	done := make(chan error, 1)

	goroutines := make(chan struct{}, maxconnections) // Ограничитель горутин
	group := sync.WaitGroup{}                         // Все запросы будут выполенены
	connections := make([]net.Conn, maxconnections)
	conmux := sync.Mutex{}

	go func() {
		<-ctx.Done()
		logger.Info("Service "+address, "Shutdowning...")
		listenerLocker.Lock()
		err := l.Close()
		//l.Close()
		l = nil
		listenerLocker.Unlock()
		conmux.Lock()
		for i, c := range connections {
			if c != nil {
				connections[i].Close()
			}
		}
		conmux.Unlock()
		logger.Info("Service "+address, "Shutdown waiting...")
		group.Wait() // Ждем завершения обработки всех запросов
		done <- err
		logger.Info("Service "+address, "Shutdown")
	}()

	for {
		listenerLocker.Lock()
		if l == nil {
			break
		}
		listenerLocker.Unlock()

		fd, err := l.Accept()
		if err != nil {
			logger.Error("Accept", err)
			continue
		}
		group.Add(1)
		goroutines <- struct{}{} // Ограничивает количество горутин
		conmux.Lock()
		ncon := -1
		for {
			for i, c := range connections {
				if c == nil {
					connections[i] = fd
					ncon = i
					break
				}
			}
			if ncon == -1 {
				time.Sleep(time.Millisecond)
				continue
			}
			break
		}
		conmux.Unlock()

		go func(conn net.Conn, nconn int) {
			logger.Debug(suckutils.ConcatTwo("Service ", address), suckutils.Concat("Open connection from ", conn.LocalAddr().String(), " to ", conn.RemoteAddr().String()))
			for {
				if err := handler(ctx, conn); err != nil {
					logger.Error("Handle "+conn.RemoteAddr().String(), err)
					break
				}
				if !connectionAlive {
					break
				}
				conn.SetDeadline(time.Time{})
			}
			logger.Debug(suckutils.ConcatTwo("Service ", address), suckutils.Concat("Connection closing from ", conn.LocalAddr().String(), " to ", conn.RemoteAddr().String()))
			if err := conn.Close(); err != nil {
				logger.Error("Close", err)
			}
			group.Done()
			<-goroutines
			conmux.Lock()
			conn.Close()
			connections[nconn] = nil
			conmux.Unlock()
			// logger.Debug("Service "+address, "end for")
		}(fd, ncon)
	}

	return <-done
}

func CreateContextWithInterruptSignal() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	go func() {
		<-stop
		cancel()
	}()
	return ctx, cancel
}

// for {
// 	conn, err := ln.Accept()
// 	if err != nil {
// 		fmt.Println(1, err)
// 		return
// 	}
// 	_, err = ws.Upgrade(conn)
// 	if err != nil {
// 		fmt.Println(2, err)
// 		return
// 	}
// 	go func() {
// 		defer conn.Close()

// 		for {
// 			header, err := ws.ReadHeader(conn)
// 			if err != nil {
// 				fmt.Println(3, err)
// 				return
// 			}

// 			payload := make([]byte, header.Length)
// 			_, err = io.ReadFull(conn, payload)
// 			if err != nil {
// 				fmt.Println(4, err)
// 				return
// 			}

// 			if header.Masked {
// 				ws.Cipher(payload, header.Mask, 0)
// 			}
// 			fmt.Println("PAYLOAD:", string(payload))
// 			// Reset the Masked flag, server frames must not be masked as
// 			// RFC6455 says.
// 			header.Masked = false

// 			// if err := ws.WriteHeader(conn, header); err != nil {
// 			// 	fmt.Println(5, err)
// 			// 	return
// 			// }
// 			if header.OpCode == ws.OpClose {
// 				fmt.Println("CLOSED")
// 				return
// 			}
// 			// if _, err := conn.Write(payload); err != nil {
// 			// 	fmt.Println(6, err)
// 			// 	return
// 			// }
// 			fr := ws.NewFrame(ws.OpText, true, []byte("hi mark"))
// 			if err = ws.WriteFrame(conn, fr); err != nil {
// 				fmt.Println(err)
// 			}
// 		}
// 	}()
// }

//func main() {

// trntlConn, err := tarantool.Connect("127.0.0.1:3301", tarantool.Opts{
// 	// User: ,
// 	// Pass: ,
// 	Timeout:       500 * time.Millisecond,
// 	Reconnect:     1 * time.Second,
// 	MaxReconnects: 4,
// })
// fmt.Println("errConn: ", err)
// //foo := auth{"login2", "password2"}
// err = trntlConn.UpsertAsync("auth", []interface{}{"login", "password"}, []interface{}{[]interface{}{"=", "password", "password"}}).Err()
// fmt.Println("errUpsert: ", err)

// var trntlAuthRec repo.TarantoolAuthTuple
// err = trntlConn.SelectTyped("auth", "secondary", 0, 1, tarantool.IterEq, []interface{}{"login", "password"}, &trntlAuthRec)
// fmt.Println("errSelect: ", err)
// fmt.Println("trntlAuthRec: ", trntlAuthRec)

// //ertrt := &tarantool.Error{Msg: suckutils.ConcatThree("Duplicate key exists in unique index 'primary' in space '", "regcodes", "'"), Code: tarantool.ErrTupleFound}

//err := trntlConn.UpsertAsync("regcodes", []interface{}{28258, "123", "asd", "asd"}, []interface{}{[]interface{}{"=", "metaid", "NEWMETAID1"}}).Err()
//fmt.Println("errUpsert:", err)

//var trntlRes tuple
//err = trntlConn.UpsertAsync("auth", []interface{}{"login", "password"}, []interface{}{[]interface{}{"=", "password", "password"}}).Err()
//err = trntlConn.UpdateAsync("regcodes", "primary", []interface{}{28258}, []interface{}{[]interface{}{"=", "metaid", "metaid"}}).Err()
//trntlConn.GetTyped("regcodes", "primary", []interface{}{28258}, &trntlRes)
//fmt.Println("err:", err)
//fmt.Println("resTrntl:", trntlRes)
//fmt.Println()

//err = trntlConn.SelectTyped("regcodes", "primary", 0, 1, tarantool.IterEq, []interface{}{28258}, &trntlRes)
// //_, err = trntlConn.Update("regcodes", "primary", []interface{}{28258}, []interface{}{[]interface{}{"=", "metaid", "h"}, []interface{}{"=", "metaname", "hh"}})

// mgoSession, err := mgo.Dial("127.0.0.1")
// if err != nil {
// 	return
// }
// mgoColl := mgoSession.DB("test").C("test")

// ch, err := mgoColl.Upsert(bson.M{"field": 750}, bson.M{"$set": bson.M{"fi": 100}, "$setOnInsert": bson.M{"field": 750}})
// fmt.Println("errinsert: ", err)

// fmt.Println("err: ", err, ch.Matched, ch.Updated)
// change := mgo.Change{
// 	Upsert:    false,
// 	Remove:    false,
// 	ReturnNew: true,
// 	Update:    bson.M{"$addToSet": bson.M{"fis": 2100}, "$setOnInsert": bson.M{"field": 1750}},
// }
// var res interface{}
// ch, err = mgoColl.Find(bson.M{"field": 1750}).Apply(change, &res)
// fmt.Println("errFindAndModify: ", err, ch.UpsertedId, "res:", res)
// auth.InitNewAuthorizer("", 0, 0)
// println("listen")
//query2 := bson.M{"type": 1, "users": bson.M{"$all": []bson.M{{"$elemMatch": bson.M{"userid": "withUserId"}}, {"$elemMatch": bson.M{"userid": "userId"}}}}}
//query2 := bson.M{"type": 1, "$or": []bson.M{{"users.0.userid": "withUserId", "users.1.userid": "userId"}, {"users.0.userid": "userId", "users.1.userid": "withUserId"}}}
//bson.M{"$elemMatch": bson.M{"userid": "userId", "type": bson.M{"$ne": 1}}}}

//err = mgoColl.Find(query2).Select(bson.M{"users.$": 1}).One(&mgores)
//bar[1] = 1
//bar[2] = 2

// var foo answer = answer{Id: "id", Text: "text"}
// answrs := make(map[string]*answer)
// answrs["id"] = &foo
// *answrs["id"] = answer{Text: "newtext"}
// fmt.Println(foo)

// ans1 := []answer{}
// ans2 := []answer{}
// ans1 = append(ans1, answer{Id: "aid1", Text: "ANS1TEXT"}, answer{Id: "aid11", Text: "ANS11TEXT"})
// ans2 = append(ans2, answer{Id: "aid2", Text: "ANS2TEXT"}, answer{Id: "aid22", Text: "ANS22TEXT"})

// holo := make(map[string]question)
// holo["qid1"] = question{Type: 1, Text: "SOMETEXT1", Answers: ans1}
// holo["qid2"] = question{Type: 2, Text: "SOMETEXT2", Answers: ans2}
// holo["qid3"] = question{Type: 3, Text: "SOMETEXT111"}

// templData, err := ioutil.ReadFile("index.html")
// if err != nil {
// 	fmt.Println("templerr1:", err)
// 	return
// }

// templ, err := template.New("index").Parse(string(templData))
// if err != nil {
// 	fmt.Println("templerr2:", err)
// 	return
// }
// var body []byte
// buf := bytes.NewBuffer(body)
// err = templ.Execute(buf, holo)
// if err != nil {
// 	fmt.Println("templerr3:", err)
// 	return
// }

// fd := buf.String()

// err = nil
// //bar := structs.Map(ffolder)
// //var b
// var inInterface map[string]interface{}
// inrec, _ := json.Marshal(ffolder)

// json.Unmarshal(inrec, &inInterface)

// fmt.Println("map: ", &inInterface)
// selector := &bson.M{"_id": "7777"} //, "metas": bson.M{"$not": bson.M{"$eq": bson.M{"id": "metaidd", "type": 5}}}}
// //query
// change := mgo.Change{
// 	Update:    bson.M{"$set": &inInterface}, //bson.M{"$pull": bson.M{"metas": bson.M{"id": "metaid2" /*, "type": bson.M{"$ne": 5}*/}}, "$currentDate": bson.M{"lastmodified": true}},
// 	Upsert:    true,
// 	ReturnNew: true,
// 	Remove:    false,
// }
// var foo interface{}
// _ = mgoSession.DB("main").C("chats").Find(selector).One(&foo)
// if err != nil {
// 	fmt.Println("errselect: ", err)
// }
// fmt.Println("foo: ", foo)
// //foo = nil
// _, err = mgoSession.DB("main").C("chats").Find(selector).Apply(change, nil)
// if err != nil {
// 	fmt.Println("errupdate: ", err)
// }
// fmt.Println("foo: ", foo)
// emailVerifyInfo := make(map[string]string, 2)

// fmt.Println("uuid: ", len(emailVerifyInfo))

// var n int = 12345
// s := strconv.Itoa(n)
// ss, er := strconv.ParseInt(s, 10, 16)
// fmt.Println("num: ", ss, er, len(s))

//}

// func (f fff) HandleError(err *errorscontainer.Error) {
// 	fmt.Println(err.Time.UTC(), err.Err.Error())
// }
