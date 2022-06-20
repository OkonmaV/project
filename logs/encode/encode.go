package encode

import (
	"encoding/binary"
	"time"

	"github.com/big-larry/suckutils"
)

const (
	TagStartSep byte = 91 // "["
	TagEndSep   byte = 93 // "]"
	TagDelim    byte = 32 // " "
)

const TagsMaxLength = 65535

var byteOrder binary.ByteOrder = binary.LittleEndian

func EncodeLog(logtype LogType, logtime time.Time, tags []byte, name, logstr string) []byte {
	log := encode(tags, logstr, name)
	log[0] = logtype.Byte()
	if !logtime.IsZero() {
		byteOrder.PutUint64(log[1:], uint64(logtime.UnixMicro()))
	}
	return log
}

func AppendTags(tags []byte, newtags ...string) []byte {
	if len(tags) == 0 {
		return encode(make([]byte, 11), "", newtags...)
	}
	return encode(tags, "", newtags...)
}

func encode(tags []byte, logstr string, newtags ...string) []byte {
	var tgs []byte
	if len(tags) == 0 {
		tgs = make([]byte, 11)
	} else {
		tgs = tags
	}
	tagslen := len(tgs)
	tslist := make([][]byte, 0, len(newtags))
	for _, tg := range newtags {
		if len(tg) > 0 {
			tb := make([]byte, 0, len(tg)+3)
			tb = append(append(append(tb, TagStartSep), tg...), TagEndSep, TagDelim)
			tagslen += len(tb)
			tslist = append(tslist, tb)
		}
	}
	if tagslen > TagsMaxLength {
		panic("tags length is out of range (lib logs/encode), last tag:" + newtags[len(newtags)-1])
	}
	log := make([]byte, 0, tagslen+len(logstr))
	log = append(log, tgs...)
	for _, t := range tslist {
		log = append(log, t...)
	}
	log = append(log, logstr...)
	byteOrder.PutUint16(log[9:], uint16(tagslen))
	return log
}

func DecodeToString(log []byte) string {
	if len(log) < 11 {
		panic("logs/encode/DecodeToString() recieved log with len less than 11") // TODO:?
	}
	return suckutils.Concat(string(TagStartSep), LogType(log[0]).String(), string(TagEndSep), " ", string(TagStartSep), time.UnixMicro(int64(byteOrder.Uint64(log[1:9]))).String(), string(TagEndSep), " ", string(log[11:]))
}

func PrintLog(log []byte) {
	println(DecodeToString(log))
}

func GetLogLvl(log []byte) LogsFlushLevel {
	return LogsFlushLevel(log[0])
}
