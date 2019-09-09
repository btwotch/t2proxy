package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"syscall"
)

const SO_ORIGINAL_DST = 80

func itod(i uint) string {
	if i == 0 {
		return "0"
	}

	// Assemble decimal in reverse order.
	var b [32]byte
	bp := len(b)
	for ; i > 0; i /= 10 {
		bp--
		b[bp] = byte(i%10) + '0'
	}

	return string(b[bp:])
}

func GetFdFromConn(l net.Conn) int {
	v := reflect.ValueOf(l)
	netFD := reflect.Indirect(reflect.Indirect(v).FieldByName("fd"))
	pfd := reflect.Indirect(netFD.FieldByName("pfd"))
	fd := int(pfd.FieldByName("Sysfd").Int())
	return fd
}

func getOriginalDst(clientConn *net.TCPConn) (ipv4 string, port uint16, err error) {
	if clientConn == nil {
		log.Printf("copy(): oops, dst is nil!")
		err = errors.New("ERR: clientConn is nil")
		return
	}

	// test if the underlying fd is nil
	remoteAddr := clientConn.RemoteAddr()
	if remoteAddr == nil {
		log.Printf("getOriginalDst(): oops, clientConn.fd is nil!")
		err = errors.New("ERR: clientConn.fd is nil")
		return
	}

	srcipport := fmt.Sprintf("%v", clientConn.RemoteAddr())

	// Get original destination
	// this is the only syscall in the Golang libs that I can find that returns 16 bytes
	// Example result: &{Multiaddr:[2 0 31 144 206 190 36 45 0 0 0 0 0 0 0 0] Interface:0}
	// port starts at the 3rd byte and is 2 bytes long (31 144 = port 8080)
	// IPv4 address starts at the 5th byte, 4 bytes long (206 190 36 45)
	addr, err := syscall.GetsockoptIPv6Mreq(GetFdFromConn(clientConn), syscall.IPPROTO_IP, SO_ORIGINAL_DST)
	log.Printf("getOriginalDst(): SO_ORIGINAL_DST=%+v\n", addr)
	if err != nil {
		log.Printf("GETORIGINALDST|%v->?->FAILEDTOBEDETERMINED|ERR: getsocketopt(SO_ORIGINAL_DST) failed: %v", srcipport, err)
		return
	}

	ipv4 = itod(uint(addr.Multiaddr[4])) + "." +
		itod(uint(addr.Multiaddr[5])) + "." +
		itod(uint(addr.Multiaddr[6])) + "." +
		itod(uint(addr.Multiaddr[7]))
	port = uint16(addr.Multiaddr[2])<<8 + uint16(addr.Multiaddr[3])

	return
}

type connectionHttpProxyBase struct {
	serverConnection io.ReadWriteCloser
	clientConnection io.ReadWriteCloser
	clientBody       io.ReadCloser
	waiter           *sync.WaitGroup
	waiterMutex      sync.Mutex
	ip               string
	port             uint16
}

type connectionHandler interface {
	handleConnection() bool
	shutDown()
	setServerConnection(serverConnection io.ReadWriteCloser)
	getServerConnection() io.ReadWriteCloser
	setClientConnection(serverConnection io.ReadWriteCloser)
	getClientConnection() io.ReadWriteCloser
	setIp(ip string)
	getIp() string
	setPort(port uint16)
	getPort() uint16
	setWaiter(waiter *sync.WaitGroup)
	getWaiter() *sync.WaitGroup
}

type connectionHttp struct {
	connectionHttpProxyBase
}
type connectionHttps struct {
	connectionHttpProxyBase
}
type connectionDirect struct {
	connectionHttpProxyBase
}

func (c *connectionHttpProxyBase) getIp() string {
	return c.ip
}

func (c *connectionHttpProxyBase) getPort() uint16 {
	return c.port
}

func (c *connectionHttpProxyBase) setServerConnection(serverConnection io.ReadWriteCloser) {
	c.serverConnection = serverConnection
}

func (c *connectionHttpProxyBase) setClientConnection(clientConnection io.ReadWriteCloser) {
	c.clientConnection = clientConnection
}

func (c *connectionHttpProxyBase) getClientConnection() io.ReadWriteCloser {
	return c.clientConnection
}

func (c *connectionHttpProxyBase) getServerConnection() io.ReadWriteCloser {
	return c.serverConnection
}

func (c *connectionHttpProxyBase) setIp(ip string) {
	c.ip = ip
}

func (c *connectionHttpProxyBase) setPort(port uint16) {
	c.port = port
}

func (c *connectionHttpProxyBase) setWaiter(waiter *sync.WaitGroup) {
	c.waiter = waiter
}

func (c *connectionHttpProxyBase) getWaiter() *sync.WaitGroup {
	return c.waiter
}

// handle traffic between proxy and server
func (c *connectionHttpProxyBase) beClient() {
	var writer io.WriteCloser
	var reader io.ReadCloser

	writer = c.getClientConnection()
	reader = c.getServerConnection()

	defer c.waiter.Done()
	for {
		n, err := io.Copy(writer, reader)
		if err != nil {
			log.Printf("io.Copy failed: %v", err)
			break
		}
		if n == 0 {
			break
		}
	}

	writer.Close()
	reader.Close()
}

func (c *connectionHttps) handleConnection() bool {
	var writer io.WriteCloser

	c.connectToProxy()

	writer = c.serverConnection

	connectString := fmt.Sprintf("CONNECT %s:%d HTTP/1.1\r\n\r\n", c.ip, c.port)
	writer.Write([]byte(connectString))
	srcReader := bufio.NewReader(c.serverConnection)
	resp, err := http.ReadResponse(srcReader, nil)
	if err != nil {
		c.waiter.Done()
		log.Printf("(CONNECT) could not parse header, got: %v", err)
		return false
	}
	fmt.Printf("(SSL) status code: %d\n", resp.StatusCode)
	go c.beClient()

	return true
}

func (c *connectionDirect) handleConnection() bool {
	remoteString := fmt.Sprintf("%s:%d", c.ip, c.port)
	log.Printf("direct remote string: %s", remoteString)

	remoteConn, err := net.Dial("tcp", remoteString)
	if err != nil {
		log.Printf("could not dial %s", remoteString)
		return false
	}

	c.setServerConnection(remoteConn)

	go c.beClient()

	return true
}

func (c *connectionHttp) handleConnection() bool {
	var writer io.WriteCloser
	var reader io.ReadCloser

	c.connectToProxy()

	writer = c.serverConnection
	reader = c.clientConnection

	srcReader := bufio.NewReader(reader)
	req, err := http.ReadRequest(srcReader)
	if err != nil {
		c.waiter.Done()
		log.Printf("could not parse request header, got: %v", err)
		return false
	}
	c.clientBody = req.Body
	log.Printf("req: %v\n", req)
	if req.Host == "" {
		log.Printf("host empty")
		return false
	}
	log.Printf("origin url string: %s\n", req.URL.String())
	u, _ := url.Parse("http://" + strings.Trim(req.Host, "/\\: ") + "/" + strings.TrimLeft(req.URL.String(), "/"))
	req.URL = u
	log.Printf("host: %v url: %v\n", req.Host, u.String())
	// I have no envy to rewrite subsequent headers
	req.Header.Set("Connection", "close")
	req.WriteProxy(writer)

	serverSrcReader := bufio.NewReader(c.serverConnection)
	resp, err := http.ReadResponse(serverSrcReader, nil)
	if err != nil {
		c.waiter.Done()
		log.Printf("could not parse response header, got: %v", err)
		return false
	}
	fmt.Printf("> resp: %v\n", resp)
	fmt.Printf(">>>>>>>>> status code: %d\n", resp.StatusCode)
	resp.Write(c.clientConnection)

	go c.beClient()

	return true
}

// handle traffic between proxy and client
func beServer(c connectionHandler) bool {

	var writer io.WriteCloser
	var reader io.ReadCloser

	defer c.getWaiter().Done()
	defer c.shutDown()
	if !c.handleConnection() {
		return false
	}

	writer = c.getServerConnection()
	reader = c.getClientConnection()

	for {
		n, err := io.Copy(writer, reader)
		if err != nil {
			log.Printf("io.Copy failed: %v", err)
			break
		}
		if n == 0 {
			break
		}
	}

	return true
}

func (c *connectionHttpProxyBase) shutDown() {
	if c.getServerConnection() != nil {
		c.getServerConnection().Close()
	}
	if c.getClientConnection() != nil {
		c.getClientConnection().Close()
	}
}

func (c *connectionHttpProxyBase) connectToProxy() {
	remoteString := "localhost:3128"
	remoteConn, err := net.Dial("tcp", remoteString)
	if err != nil {
		log.Fatalf("could not dial %s", remoteString)
	}

	c.setServerConnection(remoteConn)
}

func handleRequest(conn net.Conn) {
	var waiter sync.WaitGroup
	var c connectionHandler

	ip, port, err := getOriginalDst(conn.(*net.TCPConn))
	if err != nil {
		log.Fatalf("getOriginalDst: %v", err)
	}

	if port == 443 {
		log.Printf("Connection is https")
		c = &connectionHttps{}
	} else if port == 80 {
		log.Printf("Connection is http")
		c = &connectionHttp{}
	} else {
		log.Printf("Connection is direct")
		c = &connectionDirect{}
	}

	c.setClientConnection(conn)
	c.setIp(ip)
	c.setPort(port)
	c.setWaiter(&waiter)

	waiter.Add(2)
	beServerRet := make(chan bool)
	go func() {
		beServerRet <- beServer(c)
	}()
	fmt.Printf("beServerRet: %v\n", <-beServerRet)

	waiter.Wait()
	c.shutDown()
}

func main() {

	var err error

	l, err := net.Listen("tcp", "127.0.0.1:3129")
	if err != nil {
		log.Fatalf("could not listen: %v", err)
	}

	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatalf("accept: %v", err)
		}

		go handleRequest(conn)
	}
}
