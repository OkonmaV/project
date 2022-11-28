package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	"strconv"

	"github.com/okonma-violet/confdecoder"
)

type helloer interface {
	Hello(string)
}

type A struct {
	Str string
}
type B struct {
	str string
}

func (a *A) Hello(s string) {
	a.Str = s
	println("Helloed " + s)
}

func Create[M any, PT interface {
	helloer
	*M
}](n int) (out []*M) {
	for i := 0; i < n; i++ {
		v := PT(new(M))
		v.Hello(strconv.Itoa(i))
		out = append(out, (*M)(v))
	}
	return
}

type MessageHandler[T helloer] interface {
	HHandle(T) error
	HHandleClose(reason error)
}

func NewEpollConnector[Tmessage any,
	PTmessage interface {
		helloer
		*Tmessage
	}, TT MessageHandler[PTmessage]](messagehandler TT) {
	msg := PTmessage(new(Tmessage))
	msg.Hello("hey")

	messagehandler.HHandle(msg)
	fmt.Println(messagehandler)
}

var articulnormrx = regexp.MustCompile("[^а-яa-z0-9]")
var str = "{kangaroo,кангару}"

func normstring(s string) string {
	return articulnormrx.ReplaceAllString(strings.ToLower(s), "")
}
func readNGKarticules(filepath string) map[string]string {
	filedata, err := os.ReadFile(filepath)
	if err != nil {
		panic(err)
	}
	rx := regexp.MustCompile(`[0-9A-Za-z-]+`)
	ngkArticules := make(map[string]string)
	rows := strings.Split(string(filedata), "\n")
	for i := 0; i < len(rows); i++ {
		if arts := rx.FindAllString(rows[i], -1); len(arts) < 2 {
			continue
		} else {
			if _, ok := ngkArticules[arts[0]]; ok {
				println("doubling of articul in ngk.txt - " + arts[0])
			} else {
				ngkArticules[arts[0]] = arts[1]
			}
			if _, ok := ngkArticules[arts[1]]; ok {
				println("doubling of articul in ngk.txt - " + arts[1])
			} else {
				ngkArticules[arts[1]] = arts[0]
			}
		}
	}
	return ngkArticules
}

// substr must be lowered
func countMaxMatchLength(str string, substr []string) (count int) {
	var subcnt int
	subrs := make([][]rune, len(substr))
	for i := 0; i < len(substr); i++ {
		subrs[i] = []rune(substr[i])
	}
	r := []rune(strings.ToLower(str))
	for i, maxsubcnt := 0, 0; i < len(subrs); i, maxsubcnt = i+1, 0 {

		//fmt.Println("\n$$$ word", string(subrs[i]))
		//fmt.Println("--- subcnt-- =", 0, ",was", subcnt, ",maxsubcnt =", maxsubcnt)
		subcnt = 0
		for k, j := 0, 0; k < len(r) && j < len(subrs[i]); k++ {
			//fmt.Println("+++ compare ", string(r[:k])+"["+string(r[k])+"]"+string(r[k+1:]), "with", string(subrs[i][:j])+"["+string(subrs[i][j])+"]"+string(subrs[i][j+1:]))
			if r[k] != subrs[i][j] {
				if subcnt > maxsubcnt {
					//fmt.Println("=== maxsubcnt now", subcnt, ",was", maxsubcnt)
					maxsubcnt = subcnt
				}
				subcnt = 0
				j = 0
			} else {
				subcnt++
				j++
				//fmt.Println("--- subcnt++ =", subcnt)
			}
		}
		if subcnt > maxsubcnt {
			//fmt.Println("=+= maxsubcnt now", subcnt, ",was", maxsubcnt)
			maxsubcnt = subcnt
		}
		//fmt.Println("--- maxsubcnt =", maxsubcnt)
		if maxsubcnt > 2 {
			//fmt.Println("### count now", count+maxsubcnt, ",was", count)
			count += maxsubcnt
		}
	}
	//fmt.Println("total count", count, subcnt, string(r), substr)
	return
}
func readCSV(path string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	data, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	return data, nil
}

type config struct {
	Data         *Tts
	DataIntSlice []string
	//Num  int
}
type Tts struct {
	I int
	G string
	K []int
}

// [^-]\b\d{1,5}\b|
func main() {

	//s := &config{Data: &Tts{}}
	s := &config{}
	//pv := reflect.ValueOf(s)

	//sv := pv.Elem()
	//b := "stgh"
	//bb := reflect.ValueOf(b)
	//fv := sv.Field(0)
	//fmt.Println(s)
	//ssv := reflect.ValueOf(ss).Elem().Field(0)
	//ssv.Set(reflect.New(ssv.Type().Elem()))

	//ssv.Set(reflect.New(ssv.Type()))
	d := ";"
	vv := reflect.ValueOf([]string{"1", "2", "3", "4"})

	var dd int = 59

	for i := 0; i < 3; i++ {
		println("read")
	}
	p := "[1,2]"
	pp := strings.Trim(p, "[]")
	ppp := strings.Split(pp, ",")
	fmt.Println(ppp)

	fmt.Println(vv.Type().Elem().Kind(), rune(d[0]), string(rune(dd)))
	//fmt.Println(ssv.Kind(), bb.Kind(), vv.Index(2))

	pfd, err := confdecoder.ParseFile("config.txt")
	if err != nil {
		panic(err)
	}
	pfd.NestedStructsMode = confdecoder.NestedStructsModeTwo

	err = pfd.DecodeTo(s)

	fmt.Println(s.Data.I, s.Data.G, s.Data.K, err)

	return
	//countMaxMatchLength("Стойка стаб Стойк стабилизатора", []string{"стабилизатор", "стойк"})
	g := make(map[int][]int)
	g[1] = append(g[1])
	fmt.Println(cap(g[1]))
	return
	inds := make([]int, 0, 3)
	rxs := []*regexp.Regexp{regexp.MustCompile(`\bпроизводитель\b|\bбренд\b`), regexp.MustCompile(`артикул`), regexp.MustCompile(`наименование|название|имя`)}

	rows, err := readCSV("example.csv")
	if err != nil {
		panic(err)
	}
	fmt.Println(rows[0])

	for i := 0; i < len(rxs); i++ {
		for k := 0; k < len(rows[0]); k++ {
			if rxs[i].MatchString(strings.ToLower(rows[0][k])) {
				inds = append(inds, k)
				println(rxs[i].String(), strconv.Itoa(k))
				break
			}

		}
	}

	for i := 1; i < len(rows); i++ {
		brand_normname := strings.ToLower(rows[i][inds[0]])
		art := rows[i][inds[1]]
		name := rows[i][inds[2]]

		if len(inds) != 3 {
			panic("not found one of regs")
		}

		println("\nbrand ", brand_normname)
		println("art ", art)
		println("name ", name)
	}
	return

	ngkarts := readNGKarticules("/home/okonma/goworkspace/src/github.com/okonma-violet/spec/refs/ngk.txt")

	ii := 0
	for a1, a2 := range ngkarts {
		if ii < 5 {
			fmt.Println(ngkarts[a1], ngkarts[a2])
		} else {
			fmt.Println(ngkarts["BPR7EIX"], ngkarts["4055"])
			break
		}
		ii++
	}

	return
	rx := regexp.MustCompile(`[0-9A-Za-z-]+`)
	res := rx.FindAllString("  2097  BCPR5EP-11  ", -1)
	fmt.Println(res, len(res))

	m, _ := regexp.MatchString("производитель|бренд", "производитель")
	println(m)
	return
	f, err := os.Open("example.csv")
	if err != nil {
		panic(err)
	}
	data, err := csv.NewReader(f).ReadAll()
	if err != nil {
		panic(err)
	}
	for i, e := range data[0] {
		fmt.Println(i, e)
	}
	for i, e := range data[1] {
		fmt.Println(i, e)
	}

	f.Close()
	a := []int{1, 2, 3, 4, 5}
	fmt.Println(cap(a))
	i := 2
	a = a[:i+copy(a[i:], a[i+1:])]
	fmt.Println(cap(a))
	return
	var conn1 net.Conn
	go func() {
		println("--- grt started")
		ln, err := net.Listen("tcp", "127.0.0.1:8099")
		if err != nil {
			panic(err)
		}
		conn1, err = ln.Accept()
		if err != nil {
			panic(err)
		}
		println("--- conn accepted")
		if _, err = conn1.Write([]byte("test1")); err != nil {
			panic(err.Error())
		}
		println("--- writed test1")
	}()
	time.Sleep(time.Second)
	conn2, err := net.Dial("tcp", "127.0.0.1:8099")
	if err != nil {
		panic(err)
	}
	println("+++ dialed")
	time.Sleep(time.Second)
	buf := make([]byte, 10)

	go func() {
		time.Sleep(time.Second)
		conn2.Close()
	}()

	n, err := io.ReadFull(conn2, buf)
	if err != nil {
		er, ok := err.(*net.OpError)
		println(fmt.Sprint(errors.Is(er.Err, net.ErrClosed), ok))
		println(fmt.Sprint(reflect.TypeOf(err)))
		panic(err)
	}
	println("+++ readed", strconv.Itoa(n), "bytes, buf:", fmt.Sprint(buf), "=", string(buf))

	// buf = make([]byte, 1)
	// conn1.Close()
	// n, err = conn2.Read(buf)
	// if err != nil {
	// 	panic(err)
	// }
	// println("+++ readed", strconv.Itoa(n), "bytes, buf:", fmt.Sprint(buf), "=", string(buf))

	// go func() {
	// 	println("--- grt started")
	// 	ln, err := net.Listen("tcp", "127.0.0.1:8099")
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	c, err := ln.Accept()
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	println("--- conn accepted")
	// 	if _, err = c.Write([]byte("test1")); err != nil {
	// 		panic(err.Error())
	// 	}
	// 	println("--- writed test1, sleep")
	// 	time.Sleep(time.Second)
	// 	c.Close()
	// 	println("--- conn closed")
	// }()
	// time.Sleep(time.Second)
	// b := &B{"beforetest1"}
	// connector.SetupEpoll(nil)
	// connector.SetupPoolHandling(dynamicworkerspool.NewPool(2, 5, time.Second))
	// conn, err := net.Dial("tcp", "127.0.0.1:8099")
	// if err != nil {
	// 	panic(err)
	// }

	// rc, err := rp.NewEpollReConnector(conn, b, nil, func() error {
	// 	println("=== reconnected")
	// 	return nil
	// }) //connector.NewEpollConnector[mesag](conn, b)
	// if err != nil {
	// 	panic(err)
	// }
	// if err = rc.StartServing(); err != nil {
	// 	panic(err)
	// }
	// println("+++ reconnector created, sleep")
	// time.Sleep(time.Second * 2)
	// println("+++ wake up")

	// println("+++ b.str now is", b.str)
	// time.Sleep(time.Hour)

	//crt := Create[A](2)
	//fmt.Println(crt[1].Str)
}

func (b *B) Handle(m *mesag) error {
	println("=== new message:", m.str, ", b.str was:", b.str)
	b.str = m.str
	return nil
}

func (*B) HandleClose(r error) {
	println("=== conn closed:", r.Error())
}

type mesag struct {
	str string
}

func (m *mesag) Read(conn net.Conn) error {
	b := make([]byte, 5)
	_, err := conn.Read(b)
	if err != nil {
		return err
	}
	m.str = string(b)
	return nil
}
