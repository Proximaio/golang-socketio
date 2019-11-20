package transport

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/Proximaio/golang-socketio/logging"
	"github.com/Proximaio/golang-socketio/protocol"
)

var (
	errResponseIsNotOK       = errors.New("response body is not OK")
	errAnswerNotOpenSequence = errors.New("not opensequence answer")
	errAnswerNotOpenMessage  = errors.New("not openmessage answer")
)

// PollingClientConnection represents XHR polling client connection
type PollingClientConnection struct {
	transport *PollingClientTransport
	client    *http.Client
	url       string
	sid       string
}

// GetMessage performs a GET request to wait for the following message
func (polling *PollingClientConnection) GetMessage() (string, error) {
	logging.Log().Debug("PollingConnection.GetMessage() fired")

	resp, err := polling.client.Get(polling.url)
	if err != nil {
		logging.Log().Debug("PollingConnection.GetMessage() error polling.client.Get():", err)
		return "", err
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logging.Log().Debug("PollingConnection.GetMessage() error ioutil.ReadAll():", err)
		return "", err
	}

	bodyString := string(stripCtlAndExtFromBytes(bodyBytes))
	logging.Log().Debug("PollingConnection.GetMessage() bodyString:", bodyString)

	return bodyString, nil
}

// WriteMessage performs a POST request to send a message to server
func (polling *PollingClientConnection) WriteMessage(m string) error {
	logging.Log().Debug("PollingConnection.WriteMessage() fired, msgToWrite:", m)

	msg := withLength(m)
	buff := bytes.NewBuffer([]byte(msg))

	resp, err := polling.client.Post(polling.url, "application/json", buff)
	if err != nil {
		logging.Log().Debug("PollingConnection.WriteMessage() error polling.client.Post():", err)
		return err
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logging.Log().Debug("PollingConnection.WriteMessage() error ioutil.ReadAll():", err)
		return err
	}

	resp.Body.Close()
	bodyString := string(stripCtlAndExtFromBytes(bodyBytes))
	if bodyString != "OK" {
		return errResponseIsNotOK
	}

	return nil
}

// Close the client connection gracefully
func (polling *PollingClientConnection) Close() error {
	return polling.WriteMessage(protocol.MessageClose)
}

// PingParams returns PingInterval and PingTimeout params
func (polling *PollingClientConnection) PingParams() (time.Duration, time.Duration) {
	return polling.transport.PingInterval, polling.transport.PingTimeout
}

// PollingClientTransport represents polling client transport parameters
type PollingClientTransport struct {
	PingInterval   time.Duration
	PingTimeout    time.Duration
	ReceiveTimeout time.Duration
	SendTimeout    time.Duration

	Headers  http.Header
	sessions sessions
}

// HandleConnection for the polling client is a placeholder
func (t *PollingClientTransport) HandleConnection(w http.ResponseWriter, r *http.Request) (Connection, error) {
	return nil, nil
}

// Serve for the polling client is a placeholder
func (t *PollingClientTransport) Serve(w http.ResponseWriter, r *http.Request) {}

// SetSid for the polling client is a placeholder
func (t *PollingClientTransport) SetSid(sid string, conn Connection) {}

// openSequence represents a connection open sequence parameters
type openSequence struct {
	Sid          string        `json:"sid"`
	Upgrades     []string      `json:"upgrades"`
	PingInterval time.Duration `json:"pingInterval"`
	PingTimeout  time.Duration `json:"pingTimeout"`
}

// Connect to server, perform 3 HTTP requests in connecting sequence
func (t *PollingClientTransport) Connect(url string) (Connection, error) {
	polling := &PollingClientConnection{transport: t, client: &http.Client{}, url: url}

	resp, err := polling.client.Get(polling.url)
	if err != nil {
		logging.Log().Debug("PollingConnection.Connect() error polling.client.Get() 1:", err)
		return nil, err
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logging.Log().Debug("PollingConnection.Connect() error ioutil.ReadAll() 1:", err)
		return nil, err
	}

	resp.Body.Close()
	bodyString := string(stripCtlAndExtFromBytes(bodyBytes))
	logging.Log().Debug("PollingConnection.Connect() bodyString 1:", bodyString)

	start := strings.Index(bodyString, "{")
	end := strings.Index(bodyString, "}") + 1

	msg := bodyString[:start]
	if msg != protocol.MessageOpen {
		return nil, errAnswerNotOpenSequence
	}

	body := []byte(bodyString[start:end])
	var seq openSequence

	if err := json.Unmarshal(body, &seq); err != nil {
		logging.Log().Debug("PollingConnection.Connect() error json.Unmarshal() 1:", err)
		return nil, err
	}

	res := bodyString[end:]

	if res != protocol.MessageEmpty {
		return nil, errAnswerNotOpenMessage
	}

	polling.url += "&sid=" + seq.Sid
	polling.sid = seq.Sid

	logging.Log().Debug("PollingConnection.Connect() polling.url 1:", polling.url)

	return polling, nil
}

// DefaultPollingClientTransport returns client polling transport with default params
func DefaultPollingClientTransport() *PollingClientTransport {
	return &PollingClientTransport{
		PingInterval:   PlDefaultPingInterval,
		PingTimeout:    PlDefaultPingTimeout,
		ReceiveTimeout: PlDefaultReceiveTimeout,
		SendTimeout:    PlDefaultSendTimeout,
	}
}

func stripCtlAndExtFromBytes(in []byte) []byte {
	out := make([]byte, len(in))

	var l int

	for i := 0; i < len(in); i++ {
		c := in[i]
		if c >= 32 && c < 127 {
			out[l] = c
			l++
		}
	}

	return out[:l]
}
