package messagestypes

import (
	"strconv"
	"strings"
)

type MessageContentType byte

var (
	Unknown MessageContentType = 0
	Text    MessageContentType = 1
	Image   MessageContentType = 2
	Audio   MessageContentType = 3
	Video   MessageContentType = 4
	Vnd     MessageContentType = 5
	Error   MessageContentType = 6
)

func Parse(raw string) MessageContentType {
	if len(raw) < 4 {
		return Unknown
	}
	cropped := raw[0:2]
	switch cropped {
	case "te":
		if strings.HasPrefix(raw, "text") {
			return Text
		}
	case "im":
		if strings.HasPrefix(raw, "image") {
			return Image
		}
	case "au":
		if strings.HasPrefix(raw, "audio") {
			return Audio
		}
	case "vi":
		if strings.HasPrefix(raw, "video") {
			return Video
		}
	case "vn":
		if strings.HasPrefix(raw, "vnd") {
			return Vnd
		}
	}
	return Unknown
}

func (mct MessageContentType) String() string {
	return strconv.Itoa(int(mct))
}
