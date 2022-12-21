package main

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	"strconv"

	"github.com/jackc/pgx"
	"github.com/okonma-violet/confdecoder"
	"golang.org/x/text/encoding/charmap"
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

var str = "{kangaroo,кангару}"

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

var articulnormrx = regexp.MustCompile("[^а-яa-z0-9]|-")

func normstring(s string) string {
	return articulnormrx.ReplaceAllString(strings.ToLower(s), "")
}

func findKeywords(prods []string) []string {
	delim := rune(string(" ")[0])
	res := make([]string, 0)
loop:
	for i, lk, k, entries := 0, 0, 0, 0; i < len(prods); i, lk, k, entries = i+1, 0, 0, 0 {
		rw := []rune(strings.ToLower(strings.TrimSpace(prods[i])))

		for k < len(rw) {
			fmt.Println("word =", string(rw), "|elem =", string(rw[k]), "|entries = ", entries, "|k,lk =", k, ",", lk)
			if rw[k] == delim {
				if k-lk > 3 {
					entries++
				}
				lk = k
			}
			if entries == 3 {
				break
			}
			k++
		}
		if entries < 3 {
			lk = k
		}
		if lk > 0 {
			fmt.Println("word =", string(rw), "|entries = ", entries)
			rs := string(rw[:lk])
			for g := 0; g < len(res); g++ {
				if res[g] == rs {
					continue loop
				}
			}
			res = append(res, rs)
		}
	}
	return res
}

func unplural(str string) string {
	rs := []rune(str)
	if len(rs) < 2 {
		if len(rs) == 0 {
			return str
		}
		goto single
	}
	if (rs[len(rs)-2] == []rune("ы")[0] && (rs[len(rs)-1] == []rune("е")[0] || rs[len(rs)-1] == []rune("х")[0] || rs[len(rs)-1] == []rune("й")[0])) ||
		(rs[len(rs)-2] == []rune("и")[0] && (rs[len(rs)-1] == []rune("е")[0] || rs[len(rs)-1] == []rune("х")[0] || rs[len(rs)-1] == []rune("й")[0])) ||
		(rs[len(rs)-2] == []rune("о")[0] && (rs[len(rs)-1] == []rune("е")[0] || rs[len(rs)-1] == []rune("й")[0])) ||
		(rs[len(rs)-2] == []rune("ь")[0] && (rs[len(rs)-1] == []rune("я")[0])) ||
		(rs[len(rs)-2] == []rune("а")[0] && (rs[len(rs)-1] == []rune("я")[0])) ||
		(rs[len(rs)-2] == []rune("я")[0] && (rs[len(rs)-1] == []rune("я")[0])) ||
		(rs[len(rs)-2] == []rune("е")[0] && (rs[len(rs)-1] == []rune("е")[0])) {
		return string(rs[:len(rs)-2])

	}
single:
	if rs[len(rs)-1] == []rune("ы")[0] || rs[len(rs)-1] == []rune("а")[0] || rs[len(rs)-1] == []rune("я")[0] || rs[len(rs)-1] == []rune("и")[0] {
		return string(rs[:len(rs)-1])
	}
	return str
}

var pricerx = regexp.MustCompile("[^а-яa-z0-9.,]")
var naimrx = regexp.MustCompile(`\s{2,}`)
var artrx = regexp.MustCompile("[^а-яa-z0-9]")

func normart(s string) string {
	return artrx.ReplaceAllString(strings.ToLower(s), "")
}

func normprice(s string) string {
	return pricerx.ReplaceAllString(strings.ToLower(s), "")
}
func normnaim(s string) string {
	return naimrx.ReplaceAllString(strings.TrimSpace(s), " ")
}

func encode(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	decoder := charmap.Windows1251.NewDecoder()
	reader := decoder.Reader(f)
	b, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}
	f.Close()
	return ioutil.WriteFile(filename+"win", b, os.ModePerm)
}

type repo struct {
	db *pgx.Conn
}

func OpenRepository() *repo {
	db, err := pgx.Connect(context.Background(), "postgres://ozon:q13471347@localhost:5432/ozondb")
	if err != nil {
		panic(err)
	}
	return &repo{db: db}
}

var nonrussianrx = regexp.MustCompile(`[^а-я ]`)
var specialrx = regexp.MustCompile(`[\(\)\[\]\{\}\\\/]`)

func normspecialsrx(s string) string {
	return specialrx.ReplaceAllString(strings.ToLower(s), " ")
}
func normrussifyrx(s string) string {
	return nonrussianrx.ReplaceAllString(strings.ToLower(s), "")
}
func containsOnlyRussian(s string) bool {
	return !nonrussianrx.MatchString(s)
}
func findMatch(str string) string {
	ca := []string{"мост первый", "мост второй", "abcd     efg", "efg"}
	var kwords, words []string
	str = normspecialsrx(str)
	for i, eq, s := 0, 0, str; i < len(ca); i, eq, s = i+1, 0, str {
		if containsOnlyRussian(ca[i]) {
			s = normrussifyrx(s)
		}
		kwords = strings.Split(ca[i], " ")
		words = strings.Split(s, " ")
		fmt.Println("------------")
		for wi, ki := 0, 0; wi < len(words) && ki < len(kwords); wi++ {
			fmt.Println("|"+words[wi]+"|", "with", "|"+kwords[ki]+"|")
			if len(words[wi]) == 0 {
				continue
			}
			if len(kwords[ki]) == 0 {
				ki++
				eq++
				wi--
				continue
			}
			if strings.Compare(words[wi], kwords[ki]) == 0 {
				eq++
				ki++
			} else {
				if eq != 0 {
					ki = 0
				}
			}
		}
		if eq == len(kwords) {
			return ca[i]
		}
	}
	return ""
}

type product struct {
	id         int
	categoryid int
	supplierid int
	brandid    int
	name       string
	articul    string
	partnum    string
	price      float32
	quantity   int
	rest       int
}

func (r *repo) UpsertProduct(articul string, supplierid, brandid int, name string, partnum string, price float32, quantity, rest int, t time.Time) (*product, error) {
	id := 0
	if err := r.db.QueryRow(context.Background(), `INSERT INTO unsorted_products(articul,supplierid,brandid,name,price,partnum,quantity,rest,updated)
	values($1,$2,$3,$4,$5,$6,$7,$8,$9)
	ON CONFLICT (supplierid,name,brandid,articul) 
	DO UPDATE SET price=EXCLUDED.price,quantity=EXCLUDED.quantity,rest=EXCLUDED.rest,updated=EXCLUDED.updated
	WHERE unsorted_products.updated < EXCLUDED.updated
	RETURNING id`, articul, supplierid, brandid, name, price, partnum, quantity, rest, t).Scan(&id); err != nil {
		return nil, err
	}
	return &product{
		id:         id,
		supplierid: supplierid,
		brandid:    brandid,
		name:       name,
		price:      price,
		partnum:    partnum,
		quantity:   quantity,
		rest:       rest,
	}, nil
}
func (r *repo) AppendNormToBrand(brandid int, norm []string) error {
	_, err := r.db.Exec(context.Background(), `
	UPDATE brands
	SET norm = (select array_agg(distinct e) from unnest(norm || $2) e)
	WHERE  not norm @> $2 AND id=$1`, brandid, norm)
	return err
}

func GetProductMD5(brandid int, articul, name string) (string, error) {
	hash := md5.New()
	b := make([]byte, 4+len(articul)+len(name))
	binary.LittleEndian.PutUint32(b, uint32(brandid))
	copy(b[4:], []byte(articul))
	copy(b[4+len(articul):], []byte(name))
	fmt.Println(b)
	_, err := hash.Write(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func moveNgkToCsv() {
	outfile, err := os.Create("/home/okonma/goworkspace/src/github.com/okonma-violet/spec/docs/refs/alternative_articules.csv")
	if err != nil {
		panic(err)
	}
	defer outfile.Close()
	w := csv.NewWriter(outfile)
	w.Comma = []rune(";")[0]
	filedata, err := os.ReadFile("/home/okonma/goworkspace/src/github.com/okonma-violet/spec/docs/refs/ngk.txt")
	if err != nil {
		panic(err)
	}
	rx := regexp.MustCompile(`[0-9A-Za-z-]+`)
	readedarticules := make(map[string][]string)
	rowss := strings.Split(string(filedata), "\n")
	fmt.Println("rows num:", len(rowss))
	for i := 0; i < len(rowss); i++ {
		if arts := rx.FindAllString(rowss[i], -1); len(arts) < 2 {
			panic("len of a row < 2")
		} else {
			readedarticules[arts[1]] = append(readedarticules[arts[1]], arts[0])
		}
	}
	if err := w.Write([]string{"PRIMARY ARTICUL", "ALTERNATIVE ARTICUL"}); err != nil {
		panic(err)
	}
	for mainart, altarts := range readedarticules {
		if len(mainart) == 0 || len(altarts) == 0 {
			panic("this 1")
		}
		for _, alt := range altarts {
			if err := w.Write([]string{mainart, alt}); err != nil {
				panic(err)
			}
		}
	}
	w.Flush()
}

// [^-]\b\d{1,5}\b|
func main() {
	// file, err := os.Open("/home/okonma/goworkspace/src/github.com/okonma-violet/spec/docs/test/rawcsv/╨Я╤А╨░╨╣╤Б ╨Р╨а╨Ь╨в╨Х╨Ъ ╨Х╨║╨░╤В╨╡╤А╨╕╨╜╨▒╤Г╤А╨│.csv")
	// if err != nil {
	// 	panic(err)
	// }
	// stat, err := file.Stat()
	// if err != nil {
	// 	panic(err)
	// }
	ghj := reflect.ValueOf([]string{"1", "2", "3"}).Interface().([]string)

	fmt.Println(ghj)
	return
	gah := []string{"aaaa bbbb cccc dddd", "eeee ff g", " AAAA bbbb cccc ", " h y n g lll "}
	resgah := findKeywords(gah)
	for i, g := range resgah {
		fmt.Println(i, g+"|")
	}

	fmt.Println(normstring("abc-dfg1 2 3"), unplural("мосты"))

	return
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
