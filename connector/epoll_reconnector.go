package connector

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

type EpollReConnector struct {
	connector  *EpollConnector
	msghandler MessageHandler
	mux        sync.Mutex
	isstopped  bool

	reconAddr Addr

	doOnDial         func(net.Conn) error // right after dial, before NewEpollConnector() call
	doAfterReconnect func() error         // after StartServing() call
}

type Addr struct {
	netw    string
	address string
}

func (a Addr) Network() string {
	return a.netw
}
func (a Addr) String() string {
	return a.address
}

func (reconnector *EpollReConnector) NewMessage() MessageReader {
	return reconnector.msghandler.NewMessage()
}

func (reconnector *EpollReConnector) Handle(msg MessageReader) error {
	return reconnector.msghandler.Handle(msg)
}

func (reconnector *EpollReConnector) HandleClose(reason error) {
	reconnector.msghandler.HandleClose(reason)

	if !reconnector.isstopped {
		reconnectReq <- reconnector
	}
}

var reconnectReq chan *EpollReConnector

func InitReconnection(ctx context.Context, ticktime time.Duration, targetbufsize int, queuesize int) {
	if targetbufsize == 0 || queuesize == 0 {
		panic("target buffer size / queue size must be > 0")
	}
	if reconnectReq == nil {
		reconnectReq = make(chan *EpollReConnector, queuesize)
	} else {
		panic("reconnection re-init")
	}
	go serveReconnects(ctx, ticktime, targetbufsize)
}

// можно в канал еще пихать протокол-адрес для реконнекта, если будут возможны случаи переезда сервиса, или не мы инициировали подключение
func serveReconnects(ctx context.Context, ticktime time.Duration, targetbufsize int) { // TODO: test this
	buf := make([]*EpollReConnector, 0, targetbufsize)
	ticker := time.NewTicker(ticktime)

	for {
		select {
		case <-ctx.Done():
			return
		case req := <-reconnectReq:
			buf = append(buf, req)
		case <-ticker.C:
			for i := 0; i < len(buf); i++ {
				//if buf[i] != nil { //////////// КОСТЫЛЬ TODO: убрать

				buf[i].mux.Lock()
				if !buf[i].isstopped {
					if buf[i].connector.IsClosed() {
						conn, err := net.Dial(buf[i].reconAddr.netw, buf[i].reconAddr.address)
						if err != nil {
							buf[i].mux.Unlock()
							continue // не логается
						}
						if buf[i].doOnDial != nil {
							if err := buf[i].doOnDial(conn); err != nil {
								conn.Close()
								buf[i].mux.Unlock()
								continue
							}
						}
						conn.SetReadDeadline(time.Time{}) //TODO: нужно ли обнулять conn.readtimeout после doOnDial() ??

						newcon, err := NewEpollConnector(conn, buf[i])
						if err != nil {
							conn.Close()
							buf[i].mux.Unlock()
							continue // не логается
						}

						if err = newcon.StartServing(); err != nil {
							newcon.ClearFromCache()
							conn.Close()
							buf[i].mux.Unlock()
							continue // не логается
						}
						buf[i].connector = newcon

						if buf[i].doAfterReconnect != nil {
							if err := buf[i].doAfterReconnect(); err != nil {
								buf[i].connector.Close(err)
								buf[i].mux.Unlock()
								continue
							}
						}
					}
				}
				buf[i].mux.Unlock()
				//}
				buf = append(buf[:i], buf[i+1:]...) // трем из буфера

				i--
			}
			if cap(buf) > targetbufsize && len(buf) <= targetbufsize { // при переполнении буфера снова его уменьшаем, если к этому моменту разберемся с реконнектами // защиту от переполнения буфера ставить нельзя, иначе куда оверфловнутые реконнекты пихать
				newbuf := make([]*EpollReConnector, len(buf), targetbufsize)
				copy(newbuf, buf)
				buf = newbuf
			}
		}
	}
}

func NewEpollReConnector(conn net.Conn, messagehandler MessageHandler, doOnDial func(net.Conn) error, doAfterReconnect func() error) (*EpollReConnector, error) {
	if reconnectReq == nil {
		panic("init reconnection first")
	}

	recon := &EpollReConnector{
		msghandler:       messagehandler,
		doOnDial:         doOnDial,
		doAfterReconnect: doAfterReconnect,
		reconAddr:        Addr{netw: conn.RemoteAddr().Network(), address: conn.RemoteAddr().String()},
	}

	var err error
	if recon.connector, err = NewEpollConnector(conn, recon); err != nil {
		return nil, err
	}

	return recon, nil
}

func DialWithReconnect(netw string, addr string, messagehandler MessageHandler, doOnDial func(net.Conn) error, doAfterReconnect func() error) *EpollReConnector {
	if reconnectReq == nil {
		panic("init reconnection first")
	}
	reconn := &EpollReConnector{
		connector:        &EpollConnector{isclosed: true},
		reconAddr:        Addr{netw: netw, address: addr},
		msghandler:       messagehandler,
		doOnDial:         doOnDial,
		doAfterReconnect: doAfterReconnect,
	}
	reconnectReq <- reconn
	return reconn
}

func (reconnector *EpollReConnector) StartServing() error {
	return reconnector.connector.StartServing()
}

func (connector *EpollReConnector) ClearFromCache() {
	connector.mux.Lock()
	defer connector.mux.Unlock()

	connector.connector.ClearFromCache()
}

func (reconnector *EpollReConnector) Send(message []byte) error {
	return reconnector.connector.Send(message)
}

// doesn't stop reconnection
func (reconnector *EpollReConnector) Close(reason error) {
	reconnector.mux.Lock()
	defer reconnector.mux.Unlock()

	reconnector.connector.Close(reason)

}

// call in HandleClose() will cause deadlock
func (reconnector *EpollReConnector) IsClosed() bool {
	return reconnector.connector.IsClosed()
}

func (reconnector *EpollReConnector) RemoteAddr() net.Addr {
	return reconnector.reconAddr
}

// далее фичи для тех, кто знает что это реконнектор

var ErrAlreadyReconnected error = errors.New("already reconnected")

func (reconnector *EpollReConnector) IsReconnectStopped() bool { // только не в самой этой либе! иначе потенциальная блокировка
	reconnector.mux.Lock()
	defer reconnector.mux.Unlock()
	return reconnector.isstopped
}

// DOES NOT CLOSE CONN
func (reconnector *EpollReConnector) CancelReconnect() { // только извне! иначе потенциальная блокировка
	reconnector.mux.Lock()
	defer reconnector.mux.Unlock()
	reconnector.isstopped = true
}
