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

func handleRequest(conn net.Conn) {
	var streamWait sync.WaitGroup
	var remoteString string

	ip, port, err := getOriginalDst(conn.(*net.TCPConn))
	if err != nil {
		log.Fatalf("getOriginalDst: %v", err)
	}

	fmt.Printf("ip: %s port: %v\n", ip, port)
	//remoteString = fmt.Sprintf("%s:%d", ip, port)
	//if port == 80 {
	remoteString = "localhost:3128"
	//}
	remoteConn, err := net.Dial("tcp", remoteString)
	if err != nil {
		log.Fatalf("could not dial %s", remoteString)
	}

	streamWait.Add(2)

	streamConn := func(dst io.Writer, src io.Reader, isClient bool) {
		fmt.Printf("streamConn\n")
		if isClient {
			srcReader := bufio.NewReader(src)
			req, err := http.ReadRequest(srcReader)
			if err != nil {
				log.Printf("could not parse header, got: ")
				return
			}
			u, _ := url.Parse("http://" + req.Host)
			fmt.Printf("url: %v\n", req.URL.String())
			req.URL = u
			fmt.Printf("req: %v\n", req)
			fmt.Printf("host: %v\n", req.Host)
			req.WriteProxy(dst)
			return
		} else {
			srcReader := bufio.NewReader(src)
			res, _ := http.ReadResponse(srcReader, nil)
			fmt.Printf("res: %v\n", res)
			res.Write(dst)
		}
		for {
			n, err := io.Copy(dst, src)
			if err != nil {
				log.Printf("io.Copy failed: %v", err)
				break
			}
			if n == 0 {
				//fmt.Printf("n: %d", n)
				break
			}
		}
		streamWait.Done()
	}

	go streamConn(remoteConn, conn, true)
	go streamConn(conn, remoteConn, false)

	streamWait.Wait()
	conn.Close()
	remoteConn.Close()

	//io.WriteString(conn, "hello world\r\n\r\n")
	fmt.Printf("writestring done\n")
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
