package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"time"
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

func getFdFromConn(l net.Conn) int {
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
	addr, err := syscall.GetsockoptIPv6Mreq(getFdFromConn(clientConn), syscall.IPPROTO_IP, SO_ORIGINAL_DST)
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

type RequestHandler struct {
	devices []string
}

func (req *RequestHandler) transfer(to, from net.Conn, wg *sync.WaitGroup) {
	log.Printf("transfer: %v (%v) -> %v (%v)\n", from, from.RemoteAddr(), to, to.RemoteAddr())

	defer wg.Done()

	for {
		n, err := io.Copy(to, from)
		if err == io.EOF || n == 0 {
			to.Close()
			from.Close()
			break
		}
		if err != nil {
			log.Printf("io.Copy failed: %v", err)
			continue
		}
	}

	log.Printf("transfer done: %v (%v) -> %v (%v)\n", from, from.RemoteAddr(), to, to.RemoteAddr())
}

// copy from one socket to another manually
func (req *RequestHandler) transferDebug(to, from net.Conn, wg *sync.WaitGroup) {
	log.Printf("transfer: %v (%v) -> %v (%v)\n", from, from.RemoteAddr(), to, to.RemoteAddr())

	defer wg.Done()
	for {
		var buf []byte

		buf = make([]byte, 256)
		bytesRead, err := from.Read(buf)
		if err == io.EOF || bytesRead == 0 {
			log.Printf("end of reader: %d %v", bytesRead, err)
			break
		} else if err != nil {
			log.Fatalf("could not read: %v", err)
		}

		fmt.Printf("%+v\n", buf[:bytesRead])

		bytesWritten := 0

		for bytesWritten < bytesRead {
			fmt.Printf("writing %d bytes\n", bytesRead)
			n, err := to.Write(buf[:(bytesRead - bytesWritten)])
			fmt.Printf("written %d bytes\n", n)
			if err == io.EOF || n == 0 {
				log.Printf("end of reader: %d (%d) %v", bytesWritten, n, err)
				break
			} else if err != nil {
				log.Fatalf("could not read: %v", err)
			}
			bytesWritten += n
		}
		fmt.Printf("all written ...\n")

	}
}

func (req *RequestHandler) dialOnDevice(ip string, port uint16, device string, ctx context.Context) net.Conn {
	dialer := &net.Dialer{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				log.Printf("network: %s address: %s dest: %s:%d dev: %s\n", network, address, ip, port, device)
				err := syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, device)
				if err != nil {
					log.Printf("set sockopt failed (%s:%d dev: %s): %v", ip, port, device, err)
				}
			})
		},
		Timeout: 5 * time.Second,
	}

	log.Printf("dialing %s:%d dev: %s", ip, port, device)
	c, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		log.Printf("could not dial %s:%d: %v", ip, port, err)
	}

	return c
}

func (req *RequestHandler) dial(ip string, port uint16) net.Conn {
	var wg sync.WaitGroup

	var conn net.Conn

	contexts := make(map[string]context.CancelFunc, len(req.devices))
	var contextsMutex sync.Mutex

	wg.Add(len(req.devices))

	for _, dev := range req.devices {
		go func(dev string) {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())

			contextsMutex.Lock()
			contexts[dev] = cancel
			contextsMutex.Unlock()

			devConn := req.dialOnDevice(ip, port, dev, ctx)
			if devConn != nil {
				conn = devConn
				// cancel all other dials
				for _, otherdev := range req.devices {
					if dev == otherdev {
						continue
					}

					log.Printf("cancelling dev %s", otherdev)
					contextsMutex.Lock()
					contexts[otherdev]()
					contextsMutex.Unlock()
				}

			}
		}(dev)
	}

	wg.Wait()

	return conn
}

func (req *RequestHandler) handleRequest(conn net.Conn) {
	var wg sync.WaitGroup

	ip, port, err := getOriginalDst(conn.(*net.TCPConn))
	if err != nil {
		log.Fatalf("getOriginalDst: %v", err)
	}

	serverConn := req.dial(ip, port)

	if serverConn == nil {
		log.Printf("Calling %s:%d unsuccessful", ip, port)
		return
	}

	log.Printf("Calling %s:%d successful", ip, port)

	wg.Add(2)

	go req.transfer(conn, serverConn, &wg)
	go req.transfer(serverConn, conn, &wg)

	wg.Wait()
}

func main() {
	var err error

	l, err := net.Listen("tcp", "127.0.0.1:3129")
	if err != nil {
		log.Fatalf("could not listen: %v", err)
	}

	defer l.Close()

	devs := defaultRouteDevices()
	log.Printf("Devices with default route: %s", strings.Join(devs, ","))

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatalf("accept: %v", err)
		}

		var req RequestHandler
		req.devices = devs
		req.handleRequest(conn)
	}
}
