package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strings"
	"time"
	"unsafe"

	"github.com/jackc/pgx"
)

type ff struct {
	n string
	a []string
	b []string
}

var db *pgx.Conn

func GetBrandIdByNorm(norm string) (int, []string, error) {
	var id int
	var norms []string
	if err := db.QueryRow(context.Background(), "SELECT id,norm FROM brands WHERE norm @> ARRAY[$1]", norm).Scan(&id, &norms); err != nil { //norm@>ARRAY[$1]
		return 0, nil, err
	}
	return id, norms, nil
}

type aa struct {
	A string
	B int
	C []string
}

type bb struct {
	A *aa
}

type sps_wt struct {
	wt     int
	max_wt int // max times
}

// panics on zero wt
func foo(need_qntt int, sps []int, wts []sps_wt, pos int) bool {
	//for i := cur_pos; i < len(sps); i++ {
	//if d, tms := need_qntt%wts[i].wt, need_qntt/wts[i].wt; d == 0 {
	if pos >= len(sps) { // обошли всё дерево, выход из рекурсии
		return false
	}
	if tms := need_qntt / wts[pos].wt; tms > 0 { // можно вместить

		if tms <= wts[pos].max_wt { //хватает у поставщика
			sps[pos] = tms
			if rest_qntt := need_qntt - (sps[pos] * wts[pos].wt); rest_qntt == 0 { // заполнили без остатка
				return true
			}
		} else { // нехватает у поставщика, берем макисмально возможное
			sps[pos] = wts[pos].max_wt
		}
		fmt.Println(pos, need_qntt, sps, "\n-----------")
		need_qntt = need_qntt - (sps[pos] * wts[pos].wt)
		for ; sps[pos] >= 0; sps[pos], need_qntt = sps[pos]-1, need_qntt+wts[pos].wt { // уменьшаем исходное количество, продолжаем копать
			if foo(need_qntt, sps, wts, pos+1) {
				return true
			}
			fmt.Println(pos, need_qntt, sps)
		}
		sps[pos] = 0
		return false // вывод = невозможно
	}
	fmt.Println(pos, need_qntt, sps, "NOPE")
	return false // невозможно вместить хоть сколько то
}

const time_layout = " [01/02 15:04:05.000000] "

type locallogframe struct {
	head    []byte // [0] logtype, [1:26] time, [26:] tags with seps
	lasttag string // "name" of log record, with seps already
	body    string
}

const (
	TagStartSep byte = 91 // "["
	TagEndSep   byte = 93 // "]"
	TagDelim    byte = 32 // " "
)

func (llf *locallogframe) toString() string {
	result := make([]byte, 44+len(llf.head[26:])+len(llf.lasttag)+len(llf.body)) //5+1+3+1+5+25+len(llf.head[10:])+2+len(llf.lasttag)+2+len(llf.body)
	//var j int

	copy(result, []byte("\033[36m"))
	result[5] = TagStartSep
	copy(result[6:], []byte("DBG"))
	result[9] = TagEndSep
	copy(result[10:], []byte("\033[97m"))
	copy(result[15:], llf.head[1:26])
	copy(result[40:], llf.head[26:])
	result[40+len(llf.head[26:])] = TagDelim
	result[40+len(llf.head[26:])+1] = TagStartSep
	copy(result[40+len(llf.head[26:])+2:], []byte(llf.lasttag))
	result[40+len(llf.head[26:])+2+len(llf.lasttag)] = TagEndSep
	result[40+len(llf.head[26:])+3+len(llf.lasttag)] = TagDelim
	copy(result[40+len(llf.head[26:])+4+len(llf.lasttag):], []byte(llf.body))
	return *(*string)(unsafe.Pointer(&result))

	//return suckutils.Concat(LogType(log[0]).Colorize(), string(TagStartSep), LogType(log[0]).String(), string(TagEndSep), ColorWhite, time.UnixMicro(int64(byteOrder.Uint64(log[1:9]))).Format(time_layout), string(log[11:]))

}
func main() {

	type ghj struct {
		A int
		B string
		C string //`json:"-"`
	}
	type ghj2 struct {
		A int
		B string
		//C string `json:"-"`
	}

	gh := make(map[int]*ghj)
	gh[1] = &ghj{1, "b", "cc"}
	//gh[2] = ghj{1, "b", ""}
	fmt.Println(gh)
	ghm, err := json.Marshal(gh)
	fmt.Println(err)
	gh2 := make(map[int]*ghj2)
	err = json.Unmarshal(ghm, &gh2)
	fmt.Println(err)
	fmt.Println(gh2[1])

	return
	lfl := &locallogframe{
		lasttag: "lasttag",
		body:    "testbody123",
	}
	tgs := "[frst] [scnd]"
	lfl.head = make([]byte, len(tgs)+26)
	lfl.head[0] = []byte("a")[0]
	copy(lfl.head[1:26], []byte(time.Now().Format(time_layout)))
	copy(lfl.head[26:], []byte(tgs))
	fmt.Println(lfl.head, "||"+string(lfl.head)+"||")
	println(lfl.toString(), len("фыв"))
	var smd []byte
	smdd := []byte{1, 2, 3}
	copy(smdd, smd)
	fmt.Println(123, smdd)
	return
	adr := "127.0.0.1:8090"
	var cn net.Conn
	ln, err := net.Listen("tcp", adr)
	fmt.Println(1, err)
	go func() {
		cn, err = ln.Accept()
		fmt.Println(2, "accepted", err)
		go func() {
			buf := make([]byte, 3)
			_, err = cn.Read(buf)
			fmt.Println(3, "readed", err, buf)
		}()
		time.Sleep(time.Second * 2)
		cn.Write([]byte{1, 2, 3})
		fmt.Println(4, "writed", err)
	}()
	conn, err := net.Dial("tcp", adr)
	fmt.Println(5, "connected", conn.LocalAddr(), err)
	// go func() {
	// 	buf := make([]byte, 3)
	// 	_, err = conn.Read(buf)
	// 	fmt.Println(6, "readed", err, buf)
	// }()
	time.Sleep(time.Second * 10)

	_, err = conn.Write([]byte{4, 5, 6})
	fmt.Println(7, "writed", err)

	time.Sleep(time.Second)
	return

	// n, err := conn.Write([]byte{1, 2})
	// if err != nil {
	// 	panic(err)
	// }
	//println("n:", n)
	// altarts, err := loadAlternativeArticulesFromFile("/home/okonma/go/src/github.com/okonma-violet/spec/docs/refs/alternative_articules.csv")
	// if err != nil {
	// 	panic(err)
	// }
	// for x, alar := range altarts {
	// 	fmt.Println(x, ":\t", alar.brandname, " ", alar.arts)
	// }
	// return
	// db, err = pgx.Connect(context.Background(), "postgres://ozon:q13471347@localhost:5432/ozondb")
	// if err != nil {
	// 	panic(err)
	// }
	// id, nrms, err := GetBrandIdByNorm("hsb")

	// prdprc, err := GetProductActualPricesByProductId(999999999)
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Println(prdprc)

	// rawfile, err := os.Open("/home/okonma/go/src/github.com/okonma-violet/spec/docs/test/rawcsv/11export_Ekaterinburg_RUB_RS13826-export_Ekaterinburg_RUB_RS13826.csv")
	// if err != nil {
	// 	panic(err)
	// }
	//var def_r io.Reader = rawfile
	//def_r, err := charmap.Windows1251.NewDecoder().String("�����")
	//strconv.FormatFloat()
	//fmt.Println(def_r, err, []byte("�����"), []byte("Бренд"))
	//cnv("�����")
}
func isIntegral(val float64) bool {
	return val == float64(int(val))
}

var ErrNoParams = errors.New("no params given")

var normrx = regexp.MustCompile("[^а-яa-z0-9]")

func normstring(s string) string {
	return normrx.ReplaceAllString(strings.ToLower(s), "")
}

type productprice struct {
	Brand               string
	Articul             string
	Additional_articuls []string
	Partnum             string
	Prices              []price
}

type price struct {
	ProductId    int
	SupplierId   int
	SupplierName string
	Quantity     int
	Price        float32
	Rest         int
	Updated      time.Time
}

func GetProductActualPricesByProductId(productid int) (*productprice, error) {
	if productid != 0 {
		return getProductActualPrices("products.id=$1", []interface{}{productid})
	}
	return nil, ErrNoParams
}

func GetProductActualPricesByArtBrandname(articul, brandname string) (*productprice, error) {
	if articul != "" && brandname != "" {
		return getProductActualPrices("products.articul=$1 OR ($1=ANY(articuls.additional_articul) AND $2=ANY(brands.norm))", []interface{}{articul, normstring(brandname)})
	}
	return nil, ErrNoParams
}

func getProductActualPrices(sqlconditions string, params []interface{}) (*productprice, error) {
	if len(params) == 0 || sqlconditions == "" {
		return nil, ErrNoParams
	}

	q := `SELECT products.id,products.articul,articuls.additional_articul,brands.name,products.quantity,prices_actual.price,prices_actual.rest,suppliers.name,suppliers.id,uploads.time
	FROM products
	JOIN articuls ON products.articul=articuls.articul AND products.brandid=articuls.brandid
	JOIN prices_actual ON products.id=prices_actual.productid
	JOIN suppliers ON products.supplierid=suppliers.id
	JOIN brands ON products.brandid=brands.id
	JOIN uploads ON prices_actual.uploadid=uploads.id
	WHERE ` + sqlconditions
	fmt.Println(q)
	rows, err := db.Query(context.Background(), q, params...)
	if err != nil {
		return nil, err
	}
	res := productprice{Prices: make([]price, 0)}
	for rows.Next() {
		var prc price
		if err := rows.Scan(&prc.ProductId, &res.Articul, &res.Additional_articuls, &res.Brand, &prc.Quantity, &prc.Price, &prc.Rest, &prc.SupplierName, &prc.SupplierId, &prc.Updated); err != nil {
			rows.Close()
			return &res, err
		}
		res.Prices = append(res.Prices, prc)
	}
	return &res, nil
}

type altarticuls struct {
	brandname string
	arts      []string
}

func loadAlternativeArticulesFromFile(filepath string) ([]*altarticuls, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	r := csv.NewReader(file)
	r.ReuseRecord = true
	r.Comma = []rune(";")[0]
	r.FieldsPerRecord = 3
	if _, err = r.Read(); err != nil { // skip head
		return nil, err
	}
	counter := 0
	altarts := make([]*altarticuls, 0)
	arts_groups := make(map[string]int)
	for {
		row, err := r.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		normprim := normstring(row[0])
		normsec := normstring(row[1])
		normbrand := normstring(row[2])

		var grp int
		var ok bool
		if grp, ok = arts_groups[normprim+normbrand]; ok {
			altarts[grp].arts = append(altarts[grp].arts, normsec)
			arts_groups[normsec+normbrand] = grp
		} else if grp, ok = arts_groups[normsec+normbrand]; ok {
			altarts[grp].arts = append(altarts[grp].arts, normprim)
			arts_groups[normprim+normbrand] = grp
		} else {
			altarts = append(altarts, &altarticuls{normbrand, []string{normprim, normsec}})
			arts_groups[normsec+normbrand] = counter
			arts_groups[normprim+normbrand] = counter
			counter++
		}
	}
	return altarts, nil
}
