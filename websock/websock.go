package websock

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"

	"github.com/big-larry/suckhttp"
)

type WebSockConn struct{ conn net.Conn }

const upgrade_req_read_timeout = time.Second * 2

func Upgrade(conn net.Conn) (*WebSockConn, error) {
	req, err := suckhttp.ReadRequest(context.Background(), conn, upgrade_req_read_timeout)
	if err != nil {
		return nil, err
	}
	if req.GetMethod() != suckhttp.GET {
		return nil, errors.New("not GET")
	}
	if req.GetHeader(suckhttp.Connection) != "upgrade" {
		return nil, errors.New("non-upgrade request")
	} else if req.GetHeader("upgrade") != "websocket" {
		return nil, errors.New("no upgrade header found")
	}
	sec_ws_key := req.GetHeader("sec-websocket-key")
	if len(sec_ws_key) != 24 {
		return nil, errors.New("bad sec-key")
	}
	// len(258EAFA5-E914-47DA-95CA-C5AB0DC85B11) = 36
	ksum := make([]byte, 60) // 36+24
	copy(ksum[:24], sec_ws_key)
	copy(ksum[24:], "258EAFA5-E914-47DA-95CA-C5AB0DC85B11")
	khash := sha1.Sum(ksum)
	kbase := base64.StdEncoding.EncodeToString(khash[:])
	resp, err := suckhttp.CreateResponseMessage(101, "Deitching Protocol", []string{"Upgrade", "websocket", "Connection", "Upgrade", "Sec-Websocket-Accept", kbase}, nil)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write(resp); err != nil {
		return nil, err
	}
	return &WebSockConn{conn: conn}, nil
}

type Header struct {
}

func (wsc *WebSockConn) ReadNextHeader() (*Header, error) {
	hdr_buf := make([]byte, 2, 12)
	if _, err := wsc.conn.Read(hdr_buf); err != nil {
		return nil, err
	}
	fin := hdr_buf[0] >> 7
	opC := hdr_buf[0] & 0xF
	masked := hdr_buf[0] >> 7

	extra_hdr_size := 0
	if masked == 1 {
		extra_hdr_size += 4
	}

	payload_size := uint64(hdr_buf[1] & 0x7f)
	if payload_size == 126 {
		extra_hdr_size += 2
	} else if payload_size == 127 {
		extra_hdr_size += 8
	}
	if extra_hdr_size > 0 {
		hdr_buf = hdr_buf[:extra_hdr_size]
		if _, err := wsc.conn.Read(hdr_buf); err != nil {
			return nil, err
		}
		if payload_size == 126 {
			payload_size = uint64(binary.BigEndian.Uint16(hdr_buf[:2]))
			hdr_buf = hdr_buf[2:]
		} else if payload_size == 127 {
			payload_size = binary.BigEndian.Uint64(hdr_buf[:8])
			hdr_buf = hdr_buf[8:]
		}
	}
	payload := make([]byte, payload_size)
	if _, err := io.ReadFull(wsc.conn, payload); err != nil {
		return nil, err
	}
	if masked == 1 {
		for i := 0; i < int(payload_size); i++ {
			payload[i] ^= hdr_buf[i%4]
		}
	}

}
