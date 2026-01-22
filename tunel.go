// ./tunel -mode client -server localhost:9999 -secret my-shared-secret-key
// ./tunel -mode server -host localhost -port 9999 -server-secret my-shared-secret-key -config config.csv
// config.csv (name, server_port, client_port)
// web-app,3001,8080
package main

import (
	"crypto/rand"
	"encoding/csv"
	"encoding/gob"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

func init() {
	gob.Register(&TunnelOpen{})
	gob.Register(&Data{})
	gob.Register(&Close{})
	gob.Register(&Auth{})
}

type MessageType int

const (
	AuthMsg MessageType = iota
	TunnelOpenMsg
	DataMsg
	CloseMsg
)

type Message struct {
	Type MessageType
	Data interface{}
}

type TunnelOpen struct {
	ConnectionID string
	LocalPort    int
}

type Data struct {
	ConnectionID string
	Payload      []byte
}

type Close struct {
	ConnectionID string
}

type Auth struct {
	Secret string
}

type MessageEncoder struct {
	conn    net.Conn
	encoder *gob.Encoder
	decoder *gob.Decoder
}

func NewMessageEncoder(conn net.Conn) *MessageEncoder {
	return &MessageEncoder{
		conn:    conn,
		encoder: gob.NewEncoder(conn),
		decoder: gob.NewDecoder(conn),
	}
}

func (me *MessageEncoder) Send(msgType MessageType, data interface{}) error {
	msg := Message{
		Type: msgType,
		Data: data,
	}
	return me.encoder.Encode(msg)
}

func (me *MessageEncoder) Receive() (*Message, error) {
	var msg Message
	err := me.decoder.Decode(&msg)
	return &msg, err
}

func (me *MessageEncoder) Close() error {
	return me.conn.Close()
}

type Config struct {
	Tunnels []Tunnel
}

type ServerConfig struct {
	Host   string `toml:"host"`
	Port   int    `toml:"port"`
	Secret string `toml:"secret"`
}

type Tunnel struct {
	ServerPort int    `toml:"server_port"`
	ClientPort int    `toml:"client_port"`
	Name       string `toml:"name"`
}

func loadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	tunnels := make([]Tunnel, 0, len(records))
	for i := 0; i < len(records); i++ {
		record := records[i]
		if len(record) != 3 {
			return nil, fmt.Errorf("row %d must have exactly 3 columns", i+1)
		}
		serverPort, err := strconv.Atoi(record[1])
		if err != nil {
			return nil, fmt.Errorf("invalid server_port in row %d: %v", i+1, err)
		}
		clientPort, err := strconv.Atoi(record[2])
		if err != nil {
			return nil, fmt.Errorf("invalid client_port in row %d: %v", i+1, err)
		}
		tunnel := Tunnel{
			Name:       record[0],
			ServerPort: serverPort,
			ClientPort: clientPort,
		}
		tunnels = append(tunnels, tunnel)
	}
	return &Config{Tunnels: tunnels}, nil
}

type TunnelServer struct {
	host              string
	port              int
	secret            string
	tunnels           []Tunnel
	clients           map[string]*MessageEncoder
	clientsMu         sync.RWMutex
	activeConnections map[string]net.Conn
	connectionsMu     sync.RWMutex
}

func runServer(host string, port int, secret string, configFile string) {
	config, err := loadConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	server := &TunnelServer{
		host:              host,
		port:              port,
		secret:            secret,
		tunnels:           config.Tunnels,
		clients:           make(map[string]*MessageEncoder),
		activeConnections: make(map[string]net.Conn),
	}
	go server.startControlServer()
	for _, tunnel := range server.tunnels {
		go server.startTunnelListener(tunnel)
	}
	select {}
}

func (s *TunnelServer) startControlServer() {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to start control server: %v", err)
	}
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go s.handleClient(conn)
	}
}

func (s *TunnelServer) handleClient(conn net.Conn) {
	clientID := conn.RemoteAddr().String()
	msgEncoder := NewMessageEncoder(conn)
	msg, err := msgEncoder.Receive()
	if err != nil {
		conn.Close()
		return
	}
	if msg.Type != AuthMsg {
		conn.Close()
		return
	}
	authData := msg.Data.(*Auth)
	if authData.Secret != s.secret {
		conn.Close()
		return
	}
	s.clientsMu.Lock()
	s.clients[clientID] = msgEncoder
	s.clientsMu.Unlock()
	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, clientID)
		s.clientsMu.Unlock()
		msgEncoder.Close()
	}()
	for {
		msg, err := msgEncoder.Receive()
		if err != nil {
			break
		}
		switch msg.Type {
		case DataMsg:
			s.handleDataMessage(msg.Data.(*Data))
		case CloseMsg:
			s.handleCloseMessage(msg.Data.(*Close))
		}
	}
}

func (s *TunnelServer) startTunnelListener(tunnel Tunnel) {
	addr := fmt.Sprintf("%s:%d", s.host, tunnel.ServerPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to start tunnel listener for %s: %v", tunnel.Name, err)
	}
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go s.handleTunnelConnection(conn, tunnel)
	}
}

func (s *TunnelServer) handleTunnelConnection(conn net.Conn, tunnel Tunnel) {
	defer conn.Close()
	s.clientsMu.RLock()
	var clientEncoder *MessageEncoder
	for _, client := range s.clients {
		clientEncoder = client
		break
	}
	s.clientsMu.RUnlock()
	if clientEncoder == nil {
		return
	}
	connectionID := generateConnectionID()
	s.connectionsMu.Lock()
	s.activeConnections[connectionID] = conn
	s.connectionsMu.Unlock()
	defer func() {
		s.connectionsMu.Lock()
		delete(s.activeConnections, connectionID)
		s.connectionsMu.Unlock()
	}()
	tunnelOpen := &TunnelOpen{
		ConnectionID: connectionID,
		LocalPort:    tunnel.ClientPort,
	}
	err := clientEncoder.Send(TunnelOpenMsg, tunnelOpen)
	if err != nil {
		return
	}
	buffer := make([]byte, 4096)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading from connection %s: %v", connectionID, err)
			}
			closeMsg := &Close{ConnectionID: connectionID}
			clientEncoder.Send(CloseMsg, closeMsg)
			break
		}
		dataMsg := &Data{
			ConnectionID: connectionID,
			Payload:      buffer[:n],
		}
		err = clientEncoder.Send(DataMsg, dataMsg)
		if err != nil {
			break
		}
	}
}

func (s *TunnelServer) handleDataMessage(data *Data) {
	s.connectionsMu.RLock()
	conn, exists := s.activeConnections[data.ConnectionID]
	s.connectionsMu.RUnlock()
	if !exists {
		return
	}
	_, err := conn.Write(data.Payload)
	if err != nil {
		s.connectionsMu.Lock()
		delete(s.activeConnections, data.ConnectionID)
		s.connectionsMu.Unlock()
		conn.Close()
	}
}

func (s *TunnelServer) handleCloseMessage(close *Close) {
	s.connectionsMu.RLock()
	conn, exists := s.activeConnections[close.ConnectionID]
	s.connectionsMu.RUnlock()
	if exists {
		conn.Close()
		s.connectionsMu.Lock()
		delete(s.activeConnections, close.ConnectionID)
		s.connectionsMu.Unlock()
	}
}

func generateConnectionID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func runClient(serverAddr string, expectedSecret string) {
	for {
		conn, err := net.Dial("tcp", serverAddr)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		handleServerConnection(conn, expectedSecret)
		time.Sleep(2 * time.Second)
	}
}

type TunnelClient struct {
	msgEncoder        *MessageEncoder
	activeConnections map[string]net.Conn
	connectionsMu     sync.RWMutex
	pendingData       map[string][]*Data
	pendingMu         sync.RWMutex
}

func handleServerConnection(serverConn net.Conn, expectedSecret string) {
	defer serverConn.Close()
	client := &TunnelClient{
		msgEncoder:        NewMessageEncoder(serverConn),
		activeConnections: make(map[string]net.Conn),
		pendingData:       make(map[string][]*Data),
	}
	authMsg := &Auth{Secret: expectedSecret}
	err := client.msgEncoder.Send(AuthMsg, authMsg)
	if err != nil {
		return
	}
	for {
		msg, err := client.msgEncoder.Receive()
		if err != nil {
			break
		}
		switch msg.Type {
		case TunnelOpenMsg:
			go client.handleTunnelOpen(msg.Data.(*TunnelOpen))
		case DataMsg:
			client.handleDataMessage(msg.Data.(*Data))
		case CloseMsg:
			client.handleCloseMessage(msg.Data.(*Close))
		}
	}
	client.connectionsMu.Lock()
	for _, conn := range client.activeConnections {
		conn.Close()
	}
	client.connectionsMu.Unlock()
}

func (c *TunnelClient) handleTunnelOpen(tunnelOpen *TunnelOpen) {
	localAddr := fmt.Sprintf("localhost:%d", tunnelOpen.LocalPort)
	localConn, err := net.Dial("tcp", localAddr)
	if err != nil {
		closeMsg := &Close{ConnectionID: tunnelOpen.ConnectionID}
		c.msgEncoder.Send(CloseMsg, closeMsg)
		return
	}
	c.connectionsMu.Lock()
	c.activeConnections[tunnelOpen.ConnectionID] = localConn
	c.connectionsMu.Unlock()
	c.pendingMu.Lock()
	pendingMessages := c.pendingData[tunnelOpen.ConnectionID]
	delete(c.pendingData, tunnelOpen.ConnectionID)
	c.pendingMu.Unlock()
	for _, dataMsg := range pendingMessages {
		_, err := localConn.Write(dataMsg.Payload)
		if err != nil {
			closeMsg := &Close{ConnectionID: tunnelOpen.ConnectionID}
			c.msgEncoder.Send(CloseMsg, closeMsg)
			return
		}
	}
	defer func() {
		c.connectionsMu.Lock()
		delete(c.activeConnections, tunnelOpen.ConnectionID)
		c.connectionsMu.Unlock()
		c.pendingMu.Lock()
		delete(c.pendingData, tunnelOpen.ConnectionID)
		c.pendingMu.Unlock()
		localConn.Close()
	}()
	buffer := make([]byte, 4096)
	for {
		n, err := localConn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading from local connection %s: %v", tunnelOpen.ConnectionID, err)
			}
			closeMsg := &Close{ConnectionID: tunnelOpen.ConnectionID}
			c.msgEncoder.Send(CloseMsg, closeMsg)
			break
		}
		dataMsg := &Data{
			ConnectionID: tunnelOpen.ConnectionID,
			Payload:      buffer[:n],
		}
		err = c.msgEncoder.Send(DataMsg, dataMsg)
		if err != nil {
			break
		}
	}
}

func (c *TunnelClient) handleDataMessage(data *Data) {
	c.connectionsMu.RLock()
	conn, exists := c.activeConnections[data.ConnectionID]
	c.connectionsMu.RUnlock()
	if !exists {
		c.pendingMu.Lock()
		c.pendingData[data.ConnectionID] = append(c.pendingData[data.ConnectionID], data)
		c.pendingMu.Unlock()
		return
	}
	_, err := conn.Write(data.Payload)
	if err != nil {
		c.connectionsMu.Lock()
		delete(c.activeConnections, data.ConnectionID)
		c.connectionsMu.Unlock()
		conn.Close()
		closeMsg := &Close{ConnectionID: data.ConnectionID}
		c.msgEncoder.Send(CloseMsg, closeMsg)
	}
}

func (c *TunnelClient) handleCloseMessage(close *Close) {
	c.connectionsMu.RLock()
	conn, exists := c.activeConnections[close.ConnectionID]
	c.connectionsMu.RUnlock()
	if exists {
		conn.Close()
		c.connectionsMu.Lock()
		delete(c.activeConnections, close.ConnectionID)
		c.connectionsMu.Unlock()
	}
	c.pendingMu.Lock()
	delete(c.pendingData, close.ConnectionID)
	c.pendingMu.Unlock()
}

func main() {
	var mode = flag.String("mode", "", "Mode: 'server' or 'client'")
	var config = flag.String("config", "config.csv", "Config file path")
	var serverAddr = flag.String("server", "localhost:8080", "Server address for client mode")
	var secret = flag.String("secret", "", "Shared secret for authentication")
	var host = flag.String("host", "localhost", "Server host")
	var port = flag.Int("port", 8080, "Server port")
	var serverSecret = flag.String("server-secret", "", "Server secret")
	flag.Parse()
	switch *mode {
	case "server":
		if *serverSecret == "" {
			log.Fatal("Server secret is required")
		}
		runServer(*host, *port, *serverSecret, *config)
	case "client":
		if *secret == "" {
			log.Fatal("Client secret is required")
		}
		runClient(*serverAddr, *secret)
	default:
		log.Fatal("Invalid mode: must be 'server' or 'client'")
	}
}
