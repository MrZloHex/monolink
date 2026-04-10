package monolink

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// HandlerFunc processes an incoming message. Use req.Reply to respond via the hub.
type HandlerFunc func(req *Request)

type Option func(*Client)

func WithReconnect(interval time.Duration) Option {
	return func(c *Client) { c.reconnectInterval = interval }
}

func WithLogger(l *log.Logger) Option {
	return func(c *Client) { c.log = l }
}

func WithDialTimeout(d time.Duration) Option {
	return func(c *Client) { c.dialTimeout = d }
}

// WithTLS sets the TLS config for wss:// (client cert for mTLS, RootCAs, ServerName, etc.).
// The dialer clones this config on each dial to avoid shared mutable state across goroutines.
func WithTLS(cfg *tls.Config) Option {
	return func(c *Client) { c.tlsConfig = cfg }
}

// WithOnConnect runs after every successful dial (including reconnects).
func WithOnConnect(fn func(*Client)) Option {
	return func(c *Client) { c.onConnect = fn }
}

// WithInbox enables a buffered channel of incoming Message values (e.g. Bubble Tea).
// When the buffer is full, additional messages are dropped and logged.
func WithInbox(size int) Option {
	return func(c *Client) { c.inbox = make(chan Message, size) }
}

// Client is a WebSocket client to the MONOLITH hub (concentrator): connect, parse wire
// messages, dispatch by verb, optional inbox fan-out, and reconnect.
type Client struct {
	nodeID string
	url    string

	tlsConfig *tls.Config

	reconnectInterval time.Duration
	dialTimeout       time.Duration
	log               *log.Logger
	onConnect         func(*Client)

	conn   *websocket.Conn
	connMu sync.Mutex

	handlers  map[string]HandlerFunc
	handlerMu sync.RWMutex

	inbox chan Message

	done chan struct{}
	wg   sync.WaitGroup
}

// New creates a hub client. nodeID is uppercased and used as the wire FROM field.
func New(nodeID, url string, opts ...Option) *Client {
	c := &Client{
		nodeID:            strings.ToUpper(nodeID),
		url:               url,
		reconnectInterval: 3 * time.Second,
		dialTimeout:       5 * time.Second,
		log:               log.Default(),
		handlers:          make(map[string]HandlerFunc),
		done:              make(chan struct{}),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

func (c *Client) NodeID() string { return c.nodeID }

func (c *Client) Connected() bool {
	c.connMu.Lock()
	ok := c.conn != nil
	c.connMu.Unlock()
	return ok
}

func (c *Client) Inbox() <-chan Message { return c.inbox }

// Handle registers a handler for incoming VERB. Use "*" for catch-all.
func (c *Client) Handle(verb string, fn HandlerFunc) {
	c.handlerMu.Lock()
	c.handlers[strings.ToUpper(verb)] = fn
	c.handlerMu.Unlock()
}

// Connect dials the hub and starts the read loop. Returns after the first successful handshake or ctx cancel.
func (c *Client) Connect(ctx context.Context) error {
	if err := c.dial(ctx); err != nil {
		return fmt.Errorf("initial connection: %w", err)
	}
	c.wg.Add(1)
	go c.readLoop()
	return nil
}

// Close stops the client, closes the WebSocket, waits for the read loop, and closes the inbox channel if configured.
func (c *Client) Close() error {
	close(c.done)
	c.connMu.Lock()
	var err error
	if c.conn != nil {
		err = c.conn.Close()
		c.conn = nil
	}
	c.connMu.Unlock()
	c.wg.Wait()
	if c.inbox != nil {
		close(c.inbox)
		c.inbox = nil
	}
	return err
}

// Send writes TO:VERB:NOUN:args:FROM with FROM = this client's node ID.
func (c *Client) Send(to, verb, noun string, args ...string) error {
	wire := Encode(to, verb, noun, c.nodeID, args...)
	return c.writeRaw(wire)
}

// SendRaw writes a pre-encoded wire string.
func (c *Client) SendRaw(wire string) error {
	return c.writeRaw(wire)
}

func (c *Client) writeRaw(wire string) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	return c.conn.WriteMessage(websocket.TextMessage, []byte(wire))
}

func (c *Client) dial(ctx context.Context) error {
	dialer := websocket.Dialer{HandshakeTimeout: c.dialTimeout}
	if c.tlsConfig != nil {
		dialer.TLSClientConfig = c.tlsConfig.Clone()
	}
	conn, _, err := dialer.DialContext(ctx, c.url, nil)
	if err != nil {
		return err
	}

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	if c.onConnect != nil {
		c.onConnect(c)
	}
	return nil
}

func (c *Client) readLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.done:
			return
		default:
		}

		c.connMu.Lock()
		conn := c.conn
		c.connMu.Unlock()
		if conn == nil {
			if !c.tryReconnect() {
				return
			}
			continue
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				c.log.Printf("[%s] connection closed", c.nodeID)
				return
			}
			c.log.Printf("[%s] read error: %v", c.nodeID, err)

			c.connMu.Lock()
			c.conn = nil
			c.connMu.Unlock()

			if !c.tryReconnect() {
				return
			}
			continue
		}

		raw := strings.TrimSpace(string(data))
		if raw == "" {
			continue
		}

		msg, err := Parse(raw)
		if err != nil {
			c.log.Printf("[%s] %v", c.nodeID, err)
			continue
		}

		c.dispatch(msg)
	}
}

func (c *Client) tryReconnect() bool {
	if c.reconnectInterval <= 0 {
		return false
	}

	for {
		select {
		case <-c.done:
			return false
		case <-time.After(c.reconnectInterval):
		}

		c.log.Printf("[%s] reconnecting to %s ...", c.nodeID, c.url)
		ctx, cancel := context.WithTimeout(context.Background(), c.dialTimeout)
		err := c.dial(ctx)
		cancel()
		if err == nil {
			c.log.Printf("[%s] reconnected", c.nodeID)
			return true
		}
		c.log.Printf("[%s] reconnect failed: %v", c.nodeID, err)
	}
}

func (c *Client) dispatch(msg Message) {
	if c.inbox != nil {
		select {
		case c.inbox <- msg:
		default:
			c.log.Printf("[%s] inbox full, dropping: %s", c.nodeID, msg.Raw)
		}
	}

	verb := strings.ToUpper(msg.Verb)

	c.handlerMu.RLock()
	fn, ok := c.handlers[verb]
	if !ok {
		fn, ok = c.handlers["*"]
	}
	c.handlerMu.RUnlock()

	if ok {
		go fn(&Request{Msg: msg, client: c})
	}
}
