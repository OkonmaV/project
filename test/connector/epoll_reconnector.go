package connector

import (
	"context"
	"net"
	"sync"
	"time"
)

type EpollReConnector struct {
	connector  *EpollConnector
	msghandler MessageHandler
	mux        sync.Mutex
	isstopped  bool
}

// type msgrehandler struct {
// 	msghandler MessageHandler
// }

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
				buf[i].mux.Lock()
				if !buf[i].isstopped {
					if buf[i].connector.IsClosed() {
						conn, err := net.Dial(buf[i].connector.RemoteAddr().Network(), buf[i].connector.RemoteAddr().String()) // TODO: адрес переподключения в реконнектор пихнуть???
						if err != nil {
							buf[i].mux.Unlock()
							continue // не логается
						}
						newcon, err := NewEpollConnector(conn, buf[i])
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
			if cap(buf) > targetbufsize && len(buf) <= targetbufsize { // при переполнении буфера снова его уменьшаем, если к этому моменту разберемся с реконнектами // защиту от переполнения буфера ставить нельзя, иначе куда оверфловнутые реконнекты пихать
				newbuf := make([]*EpollReConnector, targetbufsize)
				copy(newbuf, buf)
				buf = newbuf
			}
		}
	}
}

func NewEpollReConnector(conn net.Conn, messagehandler MessageHandler) (*EpollReConnector, error) {
	if reconnectReq == nil {
		panic("init reconnector first")
	}
	recon := &EpollReConnector{msghandler: messagehandler}
	var err error
	if recon.connector, err = NewEpollConnector(conn, recon); err != nil {
		return nil, err
	}

	return recon, nil
}

func (reconnector *EpollReConnector) StartServing() error {
	return reconnector.connector.StartServing()
}

func (reconnector *EpollReConnector) Send(message []byte) error {
	return reconnector.connector.Send(message)
}

func (reconnector *EpollReConnector) Close(reason error) {
	reconnector.mux.Lock()
	defer reconnector.mux.Unlock()

	reconnector.isstopped = true

	if !reconnector.connector.IsClosed() {
		reconnector.connector.Close(reason)
	}
}

// call in HandleClose() will cause deadlock
func (reconnector *EpollReConnector) IsClosed() bool {
	return reconnector.connector.IsClosed()
}

func (reconnector *EpollReConnector) RemoteAddr() net.Addr {
	return reconnector.connector.RemoteAddr()
}

// далее фичи для тех, кто знает что это реконнектор

func (reconnector *EpollReConnector) IsReconnectStopped() bool { // только не в самой этой либе! иначе потенциальная блокировка
	reconnector.mux.Lock()
	defer reconnector.mux.Unlock()
	return reconnector.isstopped
}

// WILL NOT CLOSE CONN
func (reconnector *EpollReConnector) CancelReconnect() { // только извне! иначе потенциальная блокировка
	reconnector.mux.Lock()
	defer reconnector.mux.Unlock()
	reconnector.isstopped = true
}
