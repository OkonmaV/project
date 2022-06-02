package encode

import (
	"encoding/binary"

	"github.com/big-larry/suckutils"
)

const (
	TagStartSep byte = 91 // "["
	TagEndSep   byte = 93 // "]"
	//TagDelim byte = 32 // " "
)

const TagsMaxLength = 65535

var tagsLenByteOrder binary.ByteOrder = binary.LittleEndian

func EncodeLog(logtype LogType, tags []byte, name, logstr string) []byte {
	log := encode(tags, logstr, name)
	log[0] = logtype.Byte()
	return log
}

func AppendTags(tags []byte, newtags ...string) []byte {
	return encode(tags, "", newtags...)
}

func encode(tags []byte, logstr string, newtags ...string) []byte {
	tslen := len(tags) + len(logstr)
	if tslen == 0 {
		tslen = 3
		tags = []byte{0, 0, 0}
	}
	tslist := make([][]byte, 0, len(newtags))
	for _, tg := range newtags {
		if len(tg) > 0 {
			tb := make([]byte, 0, len(tg)+2)
			tb = append(append(append(tb, TagStartSep), tg...), TagEndSep)
			tslen += len(tb)
			tslist = append(tslist, tb)
		}
	}
	if tslen-len(logstr) > TagsMaxLength {
		panic("tags length is out of range (lib logs/encode), last tag:" + newtags[len(newtags)-1])
	}
	log := make([]byte, 0, tslen)
	log = append(log, tags...)
	for _, t := range tslist {
		log = append(log, t...)
	}
	log = append(log, logstr...)
	tagsLenByteOrder.PutUint16(log[1:], uint16(tslen))
	return log
}

func DecodeToString(log []byte) string {
	if len(log) < 4 {
		return ""
	}
	return suckutils.ConcatFour(string(TagStartSep), LogType(log[0]).String(), string(TagEndSep), string(log[3:]))
}

func PrintLog(log []byte) {
	println(DecodeToString(log))
}

func GetLogLvl(log []byte) LogsFlushLevel {
	return LogsFlushLevel(log[0])
}
