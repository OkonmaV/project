package appservice

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"project/app/protocol"
	"project/connector"
	"project/logs/logger"
	"project/types/configuratortypes"

	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
)

type address struct {
	netw string
	addr string
}

type publishers struct {
	pubs_list   map[ServiceName]*Publisher
	idserv_list map[ServiceName]*IdentityServer
	mux         sync.Mutex
	pubupdates  chan pubupdate

	configurator *configurator
	l            logger.Logger
}

type Publisher struct {
	conn        net.Conn
	servicename ServiceName
	addresses   []address
	current_ind int
	mux         sync.Mutex
	l           logger.Logger
}

type IdentityServer struct {
	pub    Publisher
	AppID  string
	Secret string
}

type OuterConns_getter interface {
	GetPublisher(name ServiceName) *Publisher
	GetIdentityServer(name ServiceName) *IdentityServer
}

// type Publisher_Sender interface {
// 	SendHTTP(request *suckhttp.Request) (response *suckhttp.Response, err error)
// }

type pubupdate struct {
	name   ServiceName
	addr   address
	status configuratortypes.ServiceStatus
}

// call before configurator created
func newPublishers(ctx context.Context, l logger.Logger, servStatus *serviceStatus, configurator *configurator, pubscheckTicktime time.Duration, pubNames []ServiceName, idservsNames []ServiceName) (*publishers, error) {
	p := &publishers{configurator: configurator, l: l, pubs_list: make(map[ServiceName]*Publisher, len(pubNames)), idserv_list: make(map[ServiceName]*IdentityServer, len(idservsNames)), pubupdates: make(chan pubupdate, 1)}
	for _, pubname := range pubNames {
		if _, err := p.newPublisher(pubname); err != nil {
			return nil, err
		}
	}
	for _, idservname := range idservsNames {
		if _, err := p.newIdentityServer(idservname); err != nil {
			return nil, err
		}
	}
	go p.publishersWorker(ctx, servStatus, pubscheckTicktime)
	return p, nil

}

func (pubs *publishers) update(pubname ServiceName, netw, addr string, status configuratortypes.ServiceStatus) {
	pubs.pubupdates <- pubupdate{name: pubname, addr: address{netw: netw, addr: addr}, status: status}
}

func (pubs *publishers) publishersWorker(ctx context.Context, servStatus *serviceStatus, pubscheckTicktime time.Duration) {
	ticker := time.NewTicker(pubscheckTicktime)
loop:
	for {
		select {
		case <-ctx.Done():
			pubs.l.Debug("publishersWorker", "context done, exiting")
			return
		case update := <-pubs.pubupdates:

			// чешем мапу
			var pub *Publisher
			pubs.mux.Lock()
			if strings.HasPrefix(string(update.name), "idserv.") {
				if p, ok := pubs.idserv_list[update.name]; ok {
					pub = &p.pub
				} else {
					pubs.mux.Unlock()
					goto not_found
				}
			} else {
				if p, ok := pubs.pubs_list[update.name]; ok {
					pub = p
				} else {
					pubs.mux.Unlock()
					goto not_found
				}
			}
			pubs.mux.Unlock()

			// если есть в мапе
			// чешем список адресов
			for i := 0; i < len(pub.addresses); i++ {
				// если нашли в списке адресов
				if update.addr.netw == pub.addresses[i].netw && update.addr.addr == pub.addresses[i].addr {
					// если нужно удалять из списка адресов
					if update.status == configuratortypes.StatusOff || update.status == configuratortypes.StatusSuspended {
						pub.mux.Lock()

						pub.addresses = append(pub.addresses[:i], pub.addresses[i+1:]...)
						if pub.conn != nil {
							if pub.conn.RemoteAddr().String() == update.addr.addr {
								pub.conn.Close()
								pubs.l.Debug("publishersWorker", suckutils.ConcatFour("due to update, closed conn to \"", string(update.name), "\" from ", update.addr.addr))
							}
						}

						if pub.current_ind > i {
							pub.current_ind--
						}

						pub.mux.Unlock()

						pubs.l.Debug("publishersWorker", suckutils.Concat("pub \"", string(update.name), "\" from ", update.addr.addr, " updated to", update.status.String()))
						continue loop

					} else if update.status == configuratortypes.StatusOn { // если нужно добавлять в список адресов = ошибка, но может ложно стрельнуть при старте сервиса, когда при подключении к конфигуратору запрос на апдейт помимо хендшейка может отправить эта горутина по тикеру
						pubs.l.Error("publishersWorker", errors.New(suckutils.Concat("recieved pubupdate to status_on for already updated status_on for \"", string(update.name), "\" from ", update.addr.addr)))
						continue loop

					} else { // если кривой апдейт
						pubs.l.Error("publishersWorker", errors.New(suckutils.Concat("unknown statuscode: ", strconv.Itoa(int(update.status)), "at update pub \"", string(update.name), "\" from ", update.addr.addr)))
						continue loop
					}
				}
			}
			// если не нашли в списке адресов

			// если нужно добавлять в список адресов
			if update.status == configuratortypes.StatusOn {
				pub.mux.Lock()
				pub.addresses = append(pub.addresses, update.addr)
				pubs.l.Debug("publishersWorker", suckutils.Concat("added new addr ", update.addr.netw, ":", update.addr.addr, " for pub ", string(pub.servicename)))
				pub.mux.Unlock()
				continue loop

			} else if update.status == configuratortypes.StatusOff || update.status == configuratortypes.StatusSuspended { // если нужно удалять из списка адресов = ошибка
				pubs.l.Error("publishersWorker", errors.New(suckutils.Concat("recieved pubupdate to status_suspend/off for already updated status_suspend/off for \"", string(update.name), "\" from ", update.addr.addr)))
				continue loop

			} else { // если кривой апдейт = ошибка
				pubs.l.Error("publishersWorker", errors.New(suckutils.Concat("unknown statuscode: ", strconv.Itoa(int(update.status)), "at update pub \"", string(update.name), "\" from ", update.addr.addr)))
				continue loop
			}

		not_found: // если нет в мапе = ошибка и отписка
			pubs.l.Error("publishersWorker", errors.New(suckutils.ConcatThree("recieved update for non-publisher \"", string(update.name), "\", sending unsubscription")))

			pubname_byte := []byte(update.name)
			message := append(append(make([]byte, 0, 2+len(update.name)), byte(configuratortypes.OperationCodeUnsubscribeFromServices), byte(len(pubname_byte))), pubname_byte...)
			if err := pubs.configurator.send(connector.FormatBasicMessage(message)); err != nil {
				pubs.l.Error("publishersWorker/configurator.Send", err)
			}

		case <-ticker.C:
			empty_pubs := make([]string, 0, len(pubs.idserv_list)+len(pubs.idserv_list))
			empty_pubs_len := 0
			//pubs.rwmux.RLock()
			pubs.mux.Lock()
			for pub_name, pub := range pubs.pubs_list {
				pub.mux.Lock()
				if len(pub.addresses) == 0 {
					empty_pubs = append(empty_pubs, string(pub_name))
					empty_pubs_len += len(pub_name)
				}
				pub.mux.Unlock()
			}
			if len(empty_pubs) != 0 {
				servStatus.setPubsStatus(false)
			} else {
				servStatus.setPubsStatus(true)
			}
			for pub_name, idserv := range pubs.idserv_list {
				idserv.pub.mux.Lock()
				if len(idserv.pub.addresses) == 0 {
					empty_pubs = append(empty_pubs, string(pub_name))
					empty_pubs_len += len(pub_name)
				}
				idserv.pub.mux.Unlock()
			}
			pubs.mux.Unlock()

			if len(empty_pubs) != 0 {
				pubs.l.Warning("publishersWorker", suckutils.ConcatTwo("no publishers with names: ", strings.Join(empty_pubs, ", ")))
				message := make([]byte, 1, 1+empty_pubs_len+len(empty_pubs))
				message[0] = byte(configuratortypes.OperationCodeSubscribeToServices)
				for _, pubname := range empty_pubs {
					//check pubname len?
					message = append(append(message, byte(len(pubname))), []byte(pubname)...)
				}
				if err := pubs.configurator.send(connector.FormatBasicMessage(message)); err != nil {
					pubs.l.Error("Publishers", errors.New(suckutils.ConcatTwo("sending subscription to configurator error: ", err.Error())))
				}
			}
		}
	}
}

func (pubs *publishers) GetAllPubNames() []ServiceName {
	pubs.mux.Lock()
	defer pubs.mux.Unlock()
	res := make([]ServiceName, 0, len(pubs.pubs_list)+len(pubs.idserv_list))
	for pubname := range pubs.pubs_list {
		res = append(res, pubname)
	}
	for idservname := range pubs.idserv_list {
		res = append(res, idservname)
	}
	return res
}

func (pubs *publishers) GetPublisher(servicename ServiceName) *Publisher {
	if pubs == nil {
		return nil
	}
	pubs.mux.Lock()
	defer pubs.mux.Unlock()
	return pubs.pubs_list[servicename]
}

func (pubs *publishers) GetIdentityServer(servicename ServiceName) *IdentityServer {
	if pubs == nil {
		return nil
	}
	pubs.mux.Lock()
	defer pubs.mux.Unlock()
	return pubs.idserv_list[servicename]
}

func (pubs *publishers) newPublisher(name ServiceName) (*Publisher, error) {
	pubs.mux.Lock()
	defer pubs.mux.Unlock()

	if len(name) == 0 {
		return nil, errors.New("empty pubname")
	}

	if _, ok := pubs.pubs_list[name]; !ok {
		p := &Publisher{servicename: name, addresses: make([]address, 0, 1), l: pubs.l.NewSubLogger(string(name))}
		pubs.pubs_list[name] = p
		return p, nil
	} else {
		return nil, errors.New("publisher already initated")
	}
}
func (pubs *publishers) newIdentityServer(name ServiceName) (*IdentityServer, error) {
	pubs.mux.Lock()
	defer pubs.mux.Unlock()

	if len(name) == 0 {
		return nil, errors.New("empty idservname")
	}

	if _, ok := pubs.idserv_list[name]; !ok {
		idsrv := &IdentityServer{pub: Publisher{servicename: name, addresses: make([]address, 0, 1), l: pubs.l.NewSubLogger(string(name))}}
		pubs.idserv_list[name] = idsrv
		return idsrv, nil
	} else {
		return nil, errors.New("idserv already initated")
	}
}

func (idserv *IdentityServer) Send(message []byte) (headers *protocol.IdentityServerMessage_Headers, response *protocol.AppMessage, err error) {
	idserv.pub.mux.Lock()
	defer idserv.pub.mux.Unlock()
	if idserv.pub.conn != nil {
		_, err = idserv.pub.conn.Write(message)
	}
	if idserv.pub.conn == nil || err != nil {
		if err != nil {
			idserv.pub.l.Error("Send", err)
		} else {
			idserv.pub.l.Debug("Conn", "not connected, reconnect")
		}
		if err = idserv.connect(); err == nil {
			if _, err = idserv.pub.conn.Write(message); err != nil {
				return
			}
		} else {
			return
		}
	}
	response = &protocol.AppMessage{}
	if err = response.Read(idserv.pub.conn); err != nil {
		return nil, nil, err
	}
	headers = &protocol.IdentityServerMessage_Headers{}
	if len(response.Headers) != 0 {
		if err = json.Unmarshal(response.Headers, headers); err != nil {
			return nil, nil, err
		}
	}
	return
}

func (pub *Publisher) Send(message []byte) (err error) {
	pub.mux.Lock()
	defer pub.mux.Unlock()
	if pub.conn != nil {
		_, err = pub.conn.Write(message)
	}
	if pub.conn == nil || err != nil {
		if err != nil {
			pub.l.Error("Send", err)
		} else {
			pub.l.Debug("Conn", "not connected, reconnect")
		}
		if err = pub.connect(); err == nil {
			_, err = pub.conn.Write(message)
			return err
		} else {
			return err
		}
	}
	return nil
}

func (pub *Publisher) SendHTTP(request *suckhttp.Request) (response *suckhttp.Response, err error) {
	pub.mux.Lock()
	defer pub.mux.Unlock()
	if pub.conn != nil {
		response, err = request.Send(context.Background(), pub.conn)
	}
	if pub.conn == nil || err != nil {
		if err != nil {
			pub.l.Error("Send", err)
		} else {
			pub.l.Debug("Conn", "not connected, reconnect")
		}
		if err = pub.connect(); err == nil {

			if response, err = request.Send(context.Background(), pub.conn); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return response, nil
}

func CreateHTTPRequestFrom(method suckhttp.HttpMethod, uri string, recievedRequest *suckhttp.Request) (*suckhttp.Request, error) {
	req, err := suckhttp.NewRequest(method, uri)
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

// no mutex inside
func (pub *Publisher) connect() error {
	if pub.conn != nil {
		pub.conn.Close()
		pub.conn = nil
	}

	for i := 0; i < len(pub.addresses); i++ {
		if pub.current_ind == len(pub.addresses) {
			pub.current_ind = 0
		}
		var err error
		if pub.conn, err = net.DialTimeout(pub.addresses[pub.current_ind].netw, pub.addresses[pub.current_ind].addr, time.Second); err != nil {
			pub.l.Error("connect/Dial", err)
			pub.current_ind++
		} else {
			goto success
		}
	}
	return errors.New("no available addresses")
success:
	pub.l.Info("Conn", suckutils.ConcatTwo("Connected to ", pub.conn.RemoteAddr().String()))
	return nil
}

// TODO: если приложение зарегалось, но проебало appid и/или secret - шо делать? + если на ~420 не смогли сохранить в файл ключи - шо делать?
func (idserv *IdentityServer) connect() error {
	if err := idserv.pub.connect(); err == nil {
		hdrs, err := json.Marshal(protocol.IdentityServerMessage_Headers{App_Id: idserv.AppID})
		if err != nil {
			panic(err)
		}
		msg, err := protocol.EncodeAppMessage(protocol.TypeAuthData, 0, time.Now().UnixNano(), hdrs, nil)
		if err != nil {
			panic(err)
		}
		// (помнить, что эта ебала дальше написана исходя из того, чтобы ошибки логались)
		if _, err = idserv.pub.conn.Write(msg); err == nil {
			resp := &protocol.AppMessage{}

			if err = resp.Read(idserv.pub.conn); err == nil {
				if resp.Type == protocol.TypeOK {
					idserv.pub.l.Debug("Conn", "auth passed")
					return nil
				} else if resp.Type == protocol.TypeError {
					if len(resp.Body) != 0 {
						if protocol.ErrorCode(resp.Body[0]) == protocol.ErrCodeNotFound {
							idserv.pub.l.Debug("Conn", "not registered, registering now")
							msg, err = protocol.EncodeAppMessage(protocol.TypeRegistration, 0, time.Now().UnixNano(), nil, []byte(thisservicename))
							if err != nil {
								panic(err)
							}
							if _, err = idserv.pub.conn.Write(msg); err == nil {
								if err = resp.Read(idserv.pub.conn); err == nil {
									if resp.Type == protocol.TypeAuthData {
										h := protocol.IdentityServerMessage_Headers{}
										if err = json.Unmarshal(resp.Headers, &h); err == nil {
											if h.App_Id != "" && h.App_Secret != "" {
												if err = idserv.saveAppKeys(); err == nil {
													idserv.AppID = h.App_Id
													idserv.Secret = h.App_Secret
													return nil
												}
												panic(err) //?????????????????????????????????????????????????????????????
											}
										}
									} else if resp.Type == protocol.TypeError && len(resp.Body) != 0 {
										return errors.New(suckutils.ConcatTwo("appauth not passed, idserv response on registration message: ErrorCode", protocol.ErrorCode(resp.Body[0]).String()))
									} else {
										return errors.New(suckutils.ConcatTwo("appauth not passed, idserv response on registration message: Type", resp.Type.String()))
									}
								}
							}
						}

					} else {
						err = errors.New("nil errCode on TypeError")
					}

				} else {
					return errors.New(suckutils.ConcatTwo("appauth not passed, idserv response on auth message: Type", resp.Type.String()))
				}
			}
		}
		return errors.New(suckutils.ConcatTwo("appauth not passed, err: ", err.Error()))
	} else {
		return err
	}
}

func (idserv *IdentityServer) saveAppKeys() error {
	file, err := suckutils.OpenConcurrentFile(context.Background(), "idservs_keys.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0664, time.Second*5)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.File.WriteString(suckutils.Concat(string(idserv.pub.servicename), " ", idserv.AppID, " ", idserv.Secret, "\n"))
	return err
}
