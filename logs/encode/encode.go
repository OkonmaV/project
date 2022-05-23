package encode

import "encoding/binary"

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

func PrintLog(log []byte) {
	println(log[3:])
}
