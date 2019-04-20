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

// handle traffic between proxy and server
func beClient(writer io.Writer, reader io.Reader, waiter *sync.WaitGroup) {
	srcReader := bufio.NewReader(reader)
	res, _ := http.ReadResponse(srcReader, nil)
	fmt.Printf("res: %v\n", res)
	res.Write(writer)
	for {
		n, err := io.Copy(writer, reader)
		if err != nil {
			log.Printf("io.Copy failed: %v", err)
			break
		}
		if n == 0 {
			//fmt.Printf("n: %d", n)
			break
		}
	}
	waiter.Done()
}

func beServer(writer io.Writer, reader io.Reader, ip string, port uint16, waiter *sync.WaitGroup) {
	var isHttps bool

	origDst := fmt.Sprintf("%s:%d", ip, port)
	fmt.Printf("origDst: %s\n", origDst)

	isHttps = port != 80
	if isHttps {
		connectString := fmt.Sprintf("CONNECT %s HTTP/1.1\r\n\r\n", origDst)
		fmt.Printf("connectString: %s\n", connectString)
		writer.Write([]byte(connectString))
		srcReader := bufio.NewReader(reader)
		req, err := http.ReadRequest(srcReader)
		if err != nil {
			log.Printf("could not parse header, got: ")
			return
		}
		fmt.Printf("req: %v\n", req)
	} else {
		srcReader := bufio.NewReader(reader)
		req, err := http.ReadRequest(srcReader)
		if err != nil {
			log.Printf("could not parse header, got: ")
			return
		}
		fmt.Printf("req: %v\n", req)
		u, _ := url.Parse("http://" + req.Host)
		fmt.Printf("url: %v\n", req.URL.String())
		req.URL = u
		fmt.Printf("host: %v\n", req.Host)
		req.WriteProxy(writer)
	}
	for {
		n, err := io.Copy(writer, reader)
		if err != nil {
			log.Printf("io.Copy failed: %v", err)
			break
		}
		if n == 0 {
			//fmt.Printf("n: %d", n)
			break
		}
	}

	waiter.Done()
	fmt.Println("waiter done")
}

func handleRequest(conn net.Conn) {
	var streamWait sync.WaitGroup
	var remoteString string

	ip, port, err := getOriginalDst(conn.(*net.TCPConn))
	if err != nil {
		log.Fatalf("getOriginalDst: %v", err)
	}

	remoteString = "localhost:3128"
	remoteConn, err := net.Dial("tcp", remoteString)
	if err != nil {
		log.Fatalf("could not dial %s", remoteString)
	}

	streamWait.Add(2)

	go beServer(remoteConn, conn, ip, port, &streamWait)
	go beClient(conn, remoteConn, &streamWait)

	streamWait.Wait()
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
