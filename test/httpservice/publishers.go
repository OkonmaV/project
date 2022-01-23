package httpservice

import (
	"context"
	"errors"
	"net"
	"project/test/connector"
	"project/test/suspender"
	"project/test/types"
	"strconv"
	"sync"

	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
)

// TODO: рассмотреть идею о white и grey листах паблишеров

type address struct {
	netw string
	addr string
}

type publishers struct {
	list       map[ServiceName]*Publisher
	mux        sync.Mutex
	pubupdates chan pubupdate

	configurator *configurator
	l            types.Logger
}

type Publisher struct {
	conn        net.Conn
	servicename ServiceName
	addresses   []address // TODO: подумать как сделать смещение (только если не через жопу)(например первый адрес мертв, и при реконнекте мы сразу со второго стартуем)

	mux sync.Mutex
	ctx context.Context
	l   types.Logger
}

type Publishers_getter interface {
	Get(name ServiceName) *Publisher
}

type pubupdate struct {
	name   ServiceName
	addr   address
	status types.ServiceStatus
}

// call before configurator created
func newPublishers(ctx context.Context, l types.Logger, ownStatus suspender.Suspendier, configurator *configurator) *publishers {
	p := &publishers{configurator: configurator, l: l, list: make(map[ServiceName]*Publisher), pubupdates: make(chan pubupdate, 1)}
	go p.publishersWorker(ctx, ownStatus)
	return p

}

func (pubs *publishers) update(pubname ServiceName, netw, addr string, status types.ServiceStatus) {
	pubs.pubupdates <- pubupdate{name: pubname, addr: address{netw: netw, addr: addr}, status: status}
}

// TODO: без тикера возникает проблема - по хорошему, мы по тику должны долбить конфигуратора подпиской, чтобы он прислал нам адреса пустых пабов(??)
// TODO: если есть вероятность того, что апдейты прилетят криво по времени (паб отрубился-подрубился и сначала прилетел апдейт на подруб, а за ним на отруб паба), или в этом времени затеряются - возвращаем тикер
func (pubs *publishers) publishersWorker(ctx context.Context, ownStatus suspender.Suspendier) {
	// ticker := time.NewTicker(checkTicktime)
	// если вернемся к тикеру-проверщику, то можно в кейсе апдейта пихать пустого паба в массив, по которому по тику будем пробегать и отправлять подписку
loop:
	for {
		select {
		case <-ctx.Done():
			pubs.l.Debug("publishersWorker", "context done, exiting")
			return
		case update := <-pubs.pubupdates:
			pubs.mux.Lock()
			// чешем мапу
			if pub, ok := pubs.list[update.name]; ok {
				// если есть в мапе
				pubs.mux.Unlock()
				// чешем список адресов
				for i := 0; i < len(pub.addresses); i++ {
					// если нашли в списке адресов
					if update.addr.netw == pub.addresses[i].netw && update.addr.addr == pub.addresses[i].addr {
						// если нужно удалять из списка адресов
						if update.status == types.StatusOff || update.status == types.StatusSuspended {
							pub.mux.Lock()

							pub.addresses = append(pub.addresses[:i], pub.addresses[i+1:]...)
							if pub.conn != nil {
								if pub.conn.RemoteAddr().String() == update.addr.addr {
									pub.conn.Close()
									pubs.l.Debug("publishersWorker", suckutils.ConcatFour("due to update closed conn to \"", string(update.name), "\" from ", update.addr.addr))
								}
							}
							// проверяем наличие адресов
							if len(pub.addresses) == 0 {
								// уходим в суспенд
								if ownStatus.OnAir() {
									ownStatus.Suspend(suckutils.ConcatThree("no available pubs \"", string(pub.servicename), "\""))
								}
								// // шлем подписку если адресов нет (=конфигуратор пришлет нам адреса пабов, если они у него есть)
								// if err := pubs.configurator.Send(connector.FormatBasicMessage(append(append(make([]byte, 0, 2+len(pub.servicename)), byte(types.OperationCodeSubscribeToServices), byte(len(pub.servicename))), []byte(pub.servicename)...))); err != nil {
								// 	pubs.l.Error("Publishers", errors.New(suckutils.ConcatTwo("sending subscription to configurator error: ", err.Error())))
								// }
							}

							pub.mux.Unlock()

							pubs.l.Debug("publishersWorker", suckutils.Concat("pub \"", string(update.name), "\" from ", update.addr.addr, " updated to", update.status.String()))
							continue loop

						} else if update.status == types.StatusOn { // если нужно добавлять в список адресов = ошибка
							pubs.l.Error("publishersWorker", errors.New(suckutils.Concat("recieved pubupdate to status_on for already updated status_on for \"", string(update.name), "\" from ", update.addr.addr)))
							continue loop

						} else { // если кривой апдейт = ошибка
							pubs.l.Error("publishersWorker", errors.New(suckutils.Concat("unknown statuscode: ", strconv.Itoa(int(update.status)), "at update pub \"", string(update.name), "\" from ", update.addr.addr)))
							continue loop
						}
					}
				}
				// если не нашли в списке адресов

				// если нужно добавлять в список адресов
				if update.status == types.StatusOn {
					pub.mux.Lock()
					pub.addresses = append(pub.addresses, update.addr)
					pub.mux.Unlock()
					continue loop

				} else if update.status == types.StatusOff || update.status == types.StatusSuspended { // если нужно удалять из списка адресов = ошибка
					pubs.l.Error("publishersWorker", errors.New(suckutils.Concat("recieved pubupdate to status_suspend/off for already updated status_suspend/off for \"", string(update.name), "\" from ", update.addr.addr)))
					continue loop

				} else { // если кривой апдейт = ошибка
					pubs.l.Error("publishersWorker", errors.New(suckutils.Concat("unknown statuscode: ", strconv.Itoa(int(update.status)), "at update pub \"", string(update.name), "\" from ", update.addr.addr)))
					continue loop
				}

			} else { // если нет в мапе = ошибка и отписка
				pubs.mux.Unlock()
				pubs.l.Error("publishersWorker", errors.New(suckutils.Concat("recieved update for non-publisher \"", string(update.name), "\", sending unsubscription")))

				pubname_byte := []byte(update.name)
				message := append(append(make([]byte, 0, 2+len(update.name)), byte(types.OperationCodeUnsubscribeFromServices), byte(len(pubname_byte))), pubname_byte...)
				if err := pubs.configurator.send(connector.FormatBasicMessage(message)); err != nil {
					pubs.l.Error("publishersWorker/configurator.Send", err)
				}
			}
			// case <-ticker.C:
			// 	empty_pubs := make([]string, 0, 1)
			// 	empty_pubs_len := 0
			// 	//pubs.rwmux.RLock()
			// 	pubs.mux.Lock()
			// 	for pub_name, pub := range pubs.list {
			// 		if len(pub.addresses) == 0 {
			// 			empty_pubs = append(empty_pubs, string(pub_name))
			// 			empty_pubs_len += len(pub_name)
			// 		}
			// 	}
			// 	pubs.mux.Unlock()
			// 	//pubs.rwmux.RUnlock()
			// 	if len(empty_pubs) != 0 {
			// 		ownStatus.Suspend(suckutils.ConcatTwo("no publishers with names: ", strings.Join(empty_pubs, ", ")))
			// 		message := make([]byte, 1, 1+empty_pubs_len+len(empty_pubs))
			// 		message[0] = byte(types.OperationCodeSubscribeToServices)
			// 		for _, pubname := range empty_pubs {
			// 			message = append(message, byte(len(pubname)))
			// 			message = append(message, []byte(pubname)...)
			// 		}
			// 		if err := pubs.configurator.Send(connector.FormatBasicMessage(message)); err != nil {
			// 			pubs.l.Error("Publishers", errors.New(suckutils.ConcatTwo("sending subscription to configurator error: ", err.Error())))
			// 		}
			// 	}

		}
	}
}

func (pubs *publishers) GetAllPubNames() []ServiceName {
	pubs.mux.Lock()
	defer pubs.mux.Unlock()
	res := make([]ServiceName, 0, len(pubs.list))
	for pubname := range pubs.list {
		res = append(res, pubname)
	}
	return res
}

func (pubs *publishers) Get(servicename ServiceName) *Publisher {
	pubs.mux.Lock()
	defer pubs.mux.Unlock()
	return pubs.list[servicename]
}

func (pubs *publishers) NewPublisher(name ServiceName) (*Publisher, error) {
	pubs.mux.Lock()
	defer pubs.mux.Unlock()

	if _, ok := pubs.list[name]; !ok {
		p := &Publisher{servicename: name, addresses: make([]address, 0, 1)}
		pubs.list[name] = p
		return p, nil
	} else {
		return nil, errors.New("publisher already initated")
	}
}

// TODO:
func (pub *Publisher) Send(message []byte) ([]byte, error) {
	return nil, errors.New("TODO")
}

func CreateHTTPRequestFrom(method suckhttp.HttpMethod, recievedRequest *suckhttp.Request) (*suckhttp.Request, error) {
	req, err := suckhttp.NewRequest(method, "")
	if err != nil {
		return nil, err
	}
	if recievedRequest == nil {
		return nil, errors.New("not set recievedRequest")
	}
	if v := recievedRequest.GetHeader("cookie"); v != "" {
		req.AddHeader("cookie", v)
	}
	return req, nil
}
func CreateHTTPRequest(method suckhttp.HttpMethod) (*suckhttp.Request, error) {
	return suckhttp.NewRequest(method, "")
}

func (pub *Publisher) SendHTTP(request *suckhttp.Request) (response *suckhttp.Response, err error) {
	pub.mux.Lock()
	defer pub.mux.Unlock()
	if pub.conn != nil {
		response, err = request.Send(pub.ctx, pub.conn)
	}
	if pub.conn == nil || err != nil {
		if err != nil {
			pub.l.Error("Send", err)
		} else {
			pub.l.Debug("Conn", "not connected, reconnect")
		}
		if err = pub.connect(); err != nil {
			if response, err = request.Send(pub.ctx, pub.conn); err != nil {
				return nil, err
			}
		}
	}
	return response, nil
}
func (pub *Publisher) connect() error {
	if pub.conn != nil {
		pub.conn.Close()
		pub.conn = nil
	}
	for _, addr := range pub.addresses {
		if conn, err := net.Dial(addr.netw, addr.addr); err != nil {
			pub.l.Error("Dial", err)
		} else {
			pub.conn = conn
		}
	}
	if pub.conn == nil {
		return errors.New("no available addresses")
	}
	pub.l.Info("Conn", suckutils.ConcatTwo("Connected to ", pub.conn.RemoteAddr().String()))
	return nil
}
