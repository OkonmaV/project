package main

import (
	"io/ioutil"
	"strings"

	"github.com/gorilla/websocket"
)

type Configurator struct {
	services map[string][]string
	//websockets map[string][]net.Conn
	upgrader websocket.Upgrader
}

type Hosts []string

func (configurator *Configurator) readsettings(settingspath string) error {
	data, err := ioutil.ReadFile(settingspath)
	if err != nil {
		return err
	}
	datastr := string(data)
	lines := strings.Split(datastr, "\n")
	for _, line := range lines {
		if len(line) < 2 || strings.HasPrefix(line, "#") {
			continue
		}
		s := strings.Split(strings.TrimSpace(line), " ")
		if len(s) < 2 {
			continue
		}
		if v, ok := configurator.services[s[0]]; ok {
			configurator.services[s[0]] = append(v, s[1:]...)
		} else {
			configurator.services[s[0]] = s[1:]
		}
	}
	return nil
}
