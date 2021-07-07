package jooki

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
)

type DiscoveryInfo struct {
	Hostname string `json:"Hostname"`
	ID string `json:"Id"`
	IP string `json:"Ip"`
	State string `json:"State"`
}

type DiscoveryPingInfo struct {
	Version string `json:"version"`
}

type JookiIP struct {
	Address string `json:"address"`
	Ping string `json:"ping"`
}

type JookiInfo struct {
	Label string `json:"label"`
	IP *JookiIP `json:"ip"`
	Live string `json:"live"`
	Version string `json:"version"`
}

type ConnectPayload struct {
	Jooki *JookiInfo `json:"jooki"`
}

func Discover() (*Client, error) {
	c := &http.Client{}
	u := &url.URL{
		Scheme: "https",
		Host: "my.jooki.rocks",
		Path: "/api/discover/v2/local_jooki",
		RawQuery: strconv.FormatFloat(rand.Float64(), 'f', -1, 64),
	}
	res, err := c.Get(u.String())
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d error in jooki device discovery", res.StatusCode)
	}
	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, err
	}
	devices := []*DiscoveryInfo{}
	err = json.Unmarshal(body, &devices)
	if err != nil {
		return nil, err
	}
	if len(devices) == 0 {
		return nil, errors.New("no jooki devices found")
	}
	for _, device := range devices {
		u = &url.URL{
			Scheme: "http",
			Host: devices[0].IP,
			Path: "/ping",
			RawQuery: strconv.FormatFloat(rand.Float64(), 'f', -1, 64),
		}
		res, err = c.Get(u.String())
		if err != nil {
			continue
		}
		if res.StatusCode != http.StatusOK {
			continue
		}
		body, err = ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			continue
		}
		dpi := DiscoveryPingInfo{}
		err = json.Unmarshal(body, &dpi)
		if err != nil {
			continue
		}
		return NewClient(device, &dpi)
	}
	return nil, errors.New("no jooki devices online")
}

type StateUpdate struct {
	Before *JookiState
	After *JookiState
	Deltas []*JookiState
}

type Client struct {
	conn mqtt.Client
	hc *http.Client
	device *DiscoveryInfo
	dpi *DiscoveryPingInfo
	lastError error
	lastState *JookiState
	stateLocker *sync.RWMutex
	awaitLocker *sync.RWMutex
	awaiters map[int]*Awaiter
}

func NewClient(device *DiscoveryInfo, dpi *DiscoveryPingInfo) (*Client, error) {
	u := &url.URL{
		Scheme: "ws",
		Host: device.IP + ":8000",
		Path: "/mqtt",
	}
	t := time.Now()
	ms := t.Unix() * 1000 + int64(t.Nanosecond() / 1e6)
	client := &Client{
		conn: nil,
		hc: &http.Client{
			Transport: http.DefaultTransport,
		},
		device: device,
		dpi: dpi,
		lastError: nil,
		lastState: &JookiState{},
		stateLocker: &sync.RWMutex{},
		awaitLocker: &sync.RWMutex{},
		awaiters: map[int]*Awaiter{},
	}
	opts := &mqtt.ClientOptions{
		Servers: []*url.URL{u},
		ClientID: fmt.Sprintf("web%d", ms),
		CleanSession: true,
		ProtocolVersion: 4,
		KeepAlive: 60,
		PingTimeout: time.Minute * 2,
		DefaultPublishHandler: func(conn mqtt.Client, m mqtt.Message) { client.onMessage(m) },
		OnConnect: func(conn mqtt.Client) { client.onConnect() },
		OnConnectionLost: func(conn mqtt.Client, err error) { client.onConnectionLost(err) },
	}
	opts.SetKeepAlive(time.Minute)
	client.conn = mqtt.NewClient(opts)
	err := client.startup()
	if err != nil {
		client.Disconnect()
		return nil, err
	}
	return client, nil
}

func (c *Client) IP() string {
	return c.device.IP
}

func (c *Client) Reconnect() (*Client, error) {
	if !c.Closed() {
		return c, nil
	}
	nc, err := NewClient(c.device, c.dpi)
	if err == nil {
		*c = *nc
		return c, nil
	}
	nc, err = Discover()
	if err == nil {
		*c = *nc
		return c, nil
	}
	return nil, err
}

func (c *Client) Disconnect() {
	if c.conn != nil {
		c.conn.Disconnect(1)
		c.conn = nil
	}
	c.cleanupAwaiters()
}

func (c *Client) Closed() bool {
	return c.conn == nil
}

func (c *Client) startup() error {
	payload := &ConnectPayload{
		Jooki: &JookiInfo{
			Label: c.device.Hostname + " *",
			IP: &JookiIP{
				Address: c.device.Hostname,
				Ping: "LIVE",
			},
			Live: c.device.Hostname,
			Version: c.dpi.Version,
		},
	}
	tok := c.conn.Connect()
	tok.Wait()
	err := tok.Error()
	if err != nil {
		return err
	}
	err = c.subscribe("/j/all/quit", func(conn mqtt.Client, m mqtt.Message) { c.onQuitMessage(m) })
	if err != nil {
		return err
	}
	err = c.subscribe("/j/web/output/state", func(conn mqtt.Client, m mqtt.Message) { c.onStateMessage(m) })
	if err != nil {
		return err
	}
	err = c.subscribe("/j/web/output/error", func(conn mqtt.Client, m mqtt.Message) { c.onErrorMessage(m) })
	if err != nil {
		return err
	}
	err = c.subscribe("/j/debug/output/pong", func(conn mqtt.Client, m mqtt.Message) { c.onPongMessage(m) })
	if err != nil {
		return err
	}
	err = c.publish("/j/debug/input/ping", nil)
	if err != nil {
		return err
	}
	err = c.publish("/j/web/input/CONNECT", payload)
	if err != nil {
		return err
	}
	err = c.publish("/j/web/input/GET_STATE", "{}")
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) subscribe(topic string, handler mqtt.MessageHandler) error {
	tok := c.conn.Subscribe(topic, 0, handler)
	ok := tok.WaitTimeout(time.Second)
	if !ok {
		return errors.New("timeout waiting for subscription ack")
	}
	return tok.Error()
}

func (c *Client) publish(topic string, msg interface{}) error {
	var data []byte
	if msg != nil {
		switch x := msg.(type) {
		case []byte:
			data = x
		case string:
			data = []byte(x)
		default:
			var err error
			data, err = json.Marshal(msg)
			if err != nil {
				return err
			}
		}
	}
	tok := c.conn.Publish(topic, 0, false, data)
	ok := tok.WaitTimeout(time.Second)
	if !ok {
		return errors.New("timeout waiting for publish ack")
	}
	return tok.Error()
}

func (c *Client) AddAwaiter() (*Awaiter, error) {
	c.awaitLocker.Lock()
	defer c.awaitLocker.Unlock()
	if c.Closed() {
		return nil, errors.New("jooki client is closed")
	}
	chid := rand.Int()
	ch := make(chan *StateUpdate, 100)
	for {
		_, ok := c.awaiters[chid]
		if ok {
			chid += 1
		} else {
			break
		}
	}
	a := NewAwaiter(c, chid, ch)
	c.awaiters[chid] = a
	return a, nil
}

func (c *Client) RemoveAwaiter(id int) {
	c.awaitLocker.Lock()
	a, ok := c.awaiters[id]
	if ok {
		delete(c.awaiters, id)
	}
	c.awaitLocker.Unlock()
	if ok && a != nil && !a.Closed() {
		a.Close()
	}
}

func (c *Client) cleanupAwaiters() {
	c.awaitLocker.Lock()
	toClose := []*Awaiter{}
	for _, a := range c.awaiters {
		toClose = append(toClose, a)
	}
	c.awaitLocker.Unlock()
	for _, a := range toClose {
		a.Close()
	}
}

func (c *Client) Await(timeout time.Duration) (*StateUpdate, error) {
	a, err := c.AddAwaiter()
	if err != nil {
		return nil, err
	}
	t := time.NewTimer(timeout)
	a.Read(t)
	return a.Close(), nil
}

func (c *Client) publishWithAwaiter(topic string, msg interface{}) (*Awaiter, error) {
	a, err := c.AddAwaiter()
	if err != nil {
		return nil, err
	}
	err = c.publish(topic, msg)
	if err != nil {
		a.Close()
		return nil, err
	}
	return a, nil
}

func (c *Client) publishAndWaitFor(topic string, msg interface{}, f func(*JookiState) bool, timeout time.Duration) (*JookiState, error) {
	a, err := c.publishWithAwaiter(topic, msg)
	if err != nil {
		return nil, err
	}
	defer a.Close()
	return a.WaitFor(f, timeout)
}

func (c *Client) onConnect() {
	log.Println("jooki connection opened")
}

func (c *Client) onConnectionLost(err error) {
	log.Println("jooki connection lost:", err)
	c.conn = nil
	c.cleanupAwaiters()
}

func (c *Client) onMessage(m mqtt.Message) {
	log.Println("default message handler", m.Topic())
	data := m.Payload()
	n := len(data)
	if n > 100 {
		data = data[:100]
	}
	log.Println(string(data))
}

func (c *Client) onQuitMessage(m mqtt.Message) {
	log.Println("quit?", string(m.Payload()))
	c.conn = nil
}

func (c *Client) onStateMessage(m mqtt.Message) {
	c.awaitLocker.RLock()
	defer c.awaitLocker.RUnlock()
	c.stateLocker.Lock()
	before := c.lastState.Clone()
	err := json.Unmarshal(m.Payload(), c.lastState)
	if err != nil {
		log.Println("error parsing jooki state:", err)
	}
	after := c.lastState.Clone()
	c.stateLocker.Unlock()
	delta := &JookiState{}
	json.Unmarshal(m.Payload(), delta)
	update := &StateUpdate{
		Before: before,
		After: after,
		Deltas: []*JookiState{delta},
	}
	for _, a := range c.awaiters {
		a.Write(update)
	}
}

func (c *Client) onErrorMessage(m mqtt.Message) {
	msg := string(m.Payload())
	c.lastError = errors.New(msg)
	log.Println("mqtt error:", msg)
}

func (c *Client) onPongMessage(m mqtt.Message) {
	log.Println("pong?", string(m.Payload()))
}

func (c *Client) GetState() *JookiState {
	c.stateLocker.RLock()
	st := c.lastState.Clone()
	c.stateLocker.RUnlock()
	return st
}

func (c *Client) Error() error {
	return c.lastError
}

