package connector

import (
	"context"
	"net"
	"sync"
	"time"
)

type EpollReConnector struct {
	connector    *EpollConnector
	msgrehandler *msgrehandler
	mux          sync.RWMutex
	isstopped    bool
}

type msgrehandler struct {
	msghandler MessageHandler
}

var reconnectReq chan *EpollReConnector
var targetbufsize = 2 // TODO

func (m *msgrehandler) NewMessage() MessageReader {
	return m.msghandler.NewMessage()
}

func (m *msgrehandler) Handle(msg MessageReader) error {
	return m.Handle(msg)
}

func (m *msgrehandler) HandleClose(reason error) {

}

// можно в канал еще пихать протокол-адрес для реконнекта, если будут возможны случаи переезда сервиса
func serveReconnects(ctx context.Context, ticktime time.Duration) { // TODO: test this
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
				buf[i].mux.Lock()
				if !buf[i].IsReconnectStopped() {
					if buf[i].connector.IsClosed() {
						conn, err := net.Dial(buf[i].connector.RemoteAddr().Network(), buf[i].connector.RemoteAddr().String()) // TODO: адрес переподключения в реконнектор пихнуть???
						if err != nil {
							buf[i].mux.Unlock()
							continue // не логается
						}
						newcon, err := NewEpollConnector(conn, buf[i].msgrehandler)
						if err != nil {
							buf[i].mux.Unlock()
							continue // не логается
						}

						if err = newcon.StartServing(); err != nil {
							buf[i].mux.Unlock()
							continue // не логается
						}
						buf[i].connector = newcon

					}
				}
				buf[i].mux.Unlock()
				buf = append(buf[:i], buf[i+1:]...) // трем из буфера
				i--
			}
			if cap(buf) > 2 && len(buf) <= 2 { // при переполнении буфера снова его уменьшаем, если к этому моменту разберемся с реконнектами // защиту от переполнения буфера ставить нельзя, иначе куда оверфловнутые реконнекты пихать
				newbuf := make([]*EpollReConnector, 0, targetbufsize)
				copy(newbuf, buf)
				buf = newbuf
			}
		}
	}
}

func NewEpollReConnector(conn net.Conn, messagehandler MessageHandler) (*EpollReConnector, error) {
	recon := &EpollReConnector{msgrehandler: &msgrehandler{msghandler: messagehandler}}
	var err error
	if recon.connector, err = NewEpollConnector(conn, recon.msgrehandler); err != nil {
		return nil, err
	}

	return recon, nil
}

func (reconnector *EpollReConnector) StartServing() error {
	return reconnector.connector.StartServing()
}

func (reconnector *EpollReConnector) Send(message []byte) error {
	return reconnector.Send(message)
}

func (reconnector *EpollReConnector) Close(reason error) {
	reconnector.mux.Lock()
	defer reconnector.mux.Unlock()

	reconnector.isstopped = true

	if !reconnector.connector.isclosed {
		reconnector.connector.Close(reason)
	}
}

// call in HandleClose() will cause deadlock
func (reconnector *EpollReConnector) IsClosed() bool {
	return reconnector.connector.isclosed
}

func (reconnector *EpollReConnector) RemoteAddr() net.Addr {
	return reconnector.connector.RemoteAddr()
}

// далее фичи для тех, кто знает что это реконнектор

func (reconnector *EpollReConnector) IsReconnectStopped() bool { // только извне! иначе потенциальная блокировка
	reconnector.mux.RLock()
	defer reconnector.mux.RUnlock()
	return reconnector.isstopped
}

func (reconnector *EpollReConnector) CancelReconnect() { // только извне! иначе потенциальная блокировка
	reconnector.mux.Lock()
	defer reconnector.mux.Unlock()
	reconnector.isstopped = true
}
