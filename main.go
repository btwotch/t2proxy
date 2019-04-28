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
	fmt.Printf("getOriginalDst(): SO_ORIGINAL_DST=%+v\n", addr)
	if err != nil {
		log.Printf("GETORIGINALDST|%v->?->FAILEDTOBEDETERMINED|ERR: getsocketopt(SO_ORIGINAL_DST) failed: %v", srcipport, err)
		return
	}
	if err != nil {
		log.Printf("GETORIGINALDST|%v->?->%v|ERR: could not create a FileConn fron clientConnFile=%+v: %v", srcipport, addr, clientConn, err)
		return
	}

	ipv4 = itod(uint(addr.Multiaddr[4])) + "." +
		itod(uint(addr.Multiaddr[5])) + "." +
		itod(uint(addr.Multiaddr[6])) + "." +
		itod(uint(addr.Multiaddr[7]))
	port = uint16(addr.Multiaddr[2])<<8 + uint16(addr.Multiaddr[3])

	return
}

type connection struct {
	serverConnection io.ReadWriter
	clientConnection io.ReadWriter
	waiter           *sync.WaitGroup
	waiterMutex      sync.Mutex
	ip               string
	port             uint16
}

// handle traffic between proxy and server
func (c *connection) beClient() {
	var writer io.Writer
	var reader io.Reader

	writer = c.clientConnection
	reader = c.serverConnection

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
}

// handle traffic between proxy and client
func (c *connection) beServer() {
	var isHttps bool

	var writer io.Writer
	var reader io.Reader

	writer = c.serverConnection
	reader = c.clientConnection
	defer c.waiter.Done()

	origDst := fmt.Sprintf("%s:%d", c.ip, c.port)
	fmt.Printf("origDst: %s\n", origDst)

	isHttps = c.port != 80
	if isHttps {
		connectString := fmt.Sprintf("CONNECT %s HTTP/1.1\r\n\r\n", origDst)
		fmt.Printf("connectString: %s\n", connectString)
		writer.Write([]byte(connectString))
		srcReader := bufio.NewReader(c.serverConnection)
		res, err := http.ReadResponse(srcReader, nil)
		if err != nil {
			c.waiter.Done()
			log.Printf("could not parse header, got: ")
			return
		}
		fmt.Printf("res: %v\n", res)
		go c.beClient()
	} else {
		srcReader := bufio.NewReader(reader)
		req, err := http.ReadRequest(srcReader)
		if err != nil {
			c.waiter.Done()
			log.Printf("could not parse header, got: ")
			return
		}
		fmt.Printf("req: %v\n", req)
		u, _ := url.Parse("http://" + req.Host)
		fmt.Printf("url: %v\n", req.URL.String())
		req.URL = u
		fmt.Printf("host: %v\n", req.Host)
		req.WriteProxy(writer)
		go c.beClient()
	}
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

}

func handleRequest(conn net.Conn) {
	var waiter sync.WaitGroup
	var remoteString string
	var c connection

	ip, port, err := getOriginalDst(conn.(*net.TCPConn))
	if err != nil {
		log.Fatalf("getOriginalDst: %v", err)
	}

	remoteString = "localhost:3128"
	remoteConn, err := net.Dial("tcp", remoteString)
	if err != nil {
		log.Fatalf("could not dial %s", remoteString)
	}

	c.serverConnection = remoteConn
	c.clientConnection = conn
	c.ip = ip
	c.port = port
	c.waiter = &waiter

	waiter.Add(2)
	go c.beServer()
	//go c.beClient()

	waiter.Wait()
	conn.Close()
	remoteConn.Close()
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
