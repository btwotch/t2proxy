package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/viper"
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
	defaultRouteDevs *defaultRouteDevices
	ips              *IpTrie
	connectTimeout   int
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
		switch err.(type) {
		case *net.OpError:
			continue
		case nil:
		default:
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
		Timeout: time.Duration(req.connectTimeout) * time.Millisecond,
	}

	log.Printf("dialing %s:%d dev: %s timeout: %d ms", ip, port, device, req.connectTimeout)
	c, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		log.Printf("could not dial %s:%d: %v", ip, port, err)
	}

	return c
}

func (req *RequestHandler) dialSequential(ip string, port uint16, devices []string) net.Conn {
	var conn net.Conn

	for _, dev := range devices {
		ctx := context.Background()

		conn = req.dialOnDevice(ip, port, dev, ctx)
		if conn != nil {
			ipBytes := net.ParseIP(ip).To4()
			req.ips.insertIPv4(ipBytes, dev)
			return conn
		}
	}

	return conn
}

func (req *RequestHandler) dialParallel(ip string, port uint16, devices []string) net.Conn {
	var wg sync.WaitGroup

	var conn net.Conn

	contexts := make(map[string]context.CancelFunc, len(devices))
	var contextsMutex sync.Mutex

	wg.Add(len(devices))

	for _, dev := range devices {
		go func(dev string) {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())

			contextsMutex.Lock()
			contexts[dev] = cancel
			contextsMutex.Unlock()

			devConn := req.dialOnDevice(ip, port, dev, ctx)
			if devConn != nil {
				conn = devConn
				ipBytes := net.ParseIP(ip).To4()
				req.ips.insertIPv4(ipBytes, dev)
				// cancel all other dials
				for _, otherdev := range devices {
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

func (req *RequestHandler) dial(ipVal net.IP, port uint16) net.Conn {
	var conn net.Conn

	redirectIpVal := req.ips.redirectFix(ipVal)
	redirectIp := redirectIpVal.String()

	if redirectIp != ipVal.String() {
		log.Printf("Rerouting %+v to %+v", ipVal, redirectIp)
	}

	fixDevice := req.ips.deviceFix(redirectIpVal)
	if fixDevice != "" {
		devices := []string{fixDevice}
		for _, dev := range req.defaultRouteDevs.get() {
			if dev != fixDevice {
				devices = append(devices, dev)
			}
		}
		conn = req.dialSequential(redirectIp, port, devices)
	} else {
		conn = req.dialParallel(redirectIp, port, req.defaultRouteDevs.get())
	}

	return conn
}

func (req *RequestHandler) handleRequest(conn net.Conn, ip net.IP, port uint16) {
	var wg sync.WaitGroup
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

func (req *RequestHandler) handleRequestTransparent(conn net.Conn) {
	ip, port, err := getOriginalDst(conn.(*net.TCPConn))
	if err != nil {
		log.Fatalf("getOriginalDst: %v", err)
	}

	req.handleRequest(conn, net.ParseIP(ip), port)
}

func (req *RequestHandler) handleRequestSocks4(conn net.Conn) {
	requestBuf := make([]byte, 256)
	bytesRead, err := conn.Read(requestBuf)
	if err != nil {
		log.Fatalf("read: %v", err)
	}

	if bytesRead < 7 {
		log.Printf("request message too short")
		conn.Close()
		return
	}
	if requestBuf[bytesRead-1] != 0x0 {
		log.Printf("request too long")
		conn.Close()
		return
	}

	if requestBuf[0] != 0x4 {
		log.Printf("only socks4 supported - sorry!")
		conn.Close()
		return
	}
	if requestBuf[1] != 0x1 {
		log.Printf("unsupported command 0x%x", requestBuf[1])
		conn.Close()
		return
	}
	port := uint16(requestBuf[2])*256 + uint16(requestBuf[3])

	ip := net.IPv4(requestBuf[4], requestBuf[5], requestBuf[6], requestBuf[7])
	log.Printf("socks connection to %+v:%d\n", ip, port)

	responseBuf := make([]byte, 8)
	responseBuf[0] = 0x0
	responseBuf[1] = 0x5A
	responseBuf[2] = 0x0
	responseBuf[3] = 0x0
	responseBuf[4] = 0x0
	responseBuf[5] = 0x0
	responseBuf[6] = 0x0
	responseBuf[7] = 0x0

	conn.Write(responseBuf)

	req.handleRequest(conn, ip, port)
}

func serverStatus(ip *IpTrie) {
	srv := &http.Server{Addr: "127.0.0.1:3130"}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ipDump := ip.dump()
		io.WriteString(w, ipDump)
	})

	log.Fatal(srv.ListenAndServe())
}

func updateDefaultRouteDevs(drd *defaultRouteDevices) {

	updateMs := viper.GetInt("device-update-interval-ms")
	if updateMs == 0 {
		return
	}

	for {
		time.Sleep(time.Duration(updateMs) * time.Millisecond)
		drd.update()
		log.Printf("Updated devices with default route: %s", strings.Join(drd.get(), ", "))
	}
}

func main() {
	var err error

	viper.SetConfigName("t2proxy")
	viper.AddConfigPath("/etc")
	viper.AddConfigPath("$HOME/.t2proxy")
	viper.AddConfigPath(".")

	viper.SetDefault("connect-timeout-ms", 1000)
	viper.SetDefault("device-update-interval-ms", 30000)

	err = viper.ReadInConfig()
	if err != nil {
		log.Fatalf("could not read config: %v", err)
	}

	it := makeIpTrie("", 0)

	for k, v := range viper.GetStringMapString("fixed-devices") {
		it.insertHostFix(k, v)
	}

	for k, v := range viper.GetStringMapString("redirect-ips") {
		it.insertRedirectFix(k, net.ParseIP(v).To4())
	}

	defaultDevs := makeDefaultRouteDevices(viper.GetStringSlice("devices"))
	log.Printf("Devices with default route: %s", strings.Join(defaultDevs.get(), ", "))
	go updateDefaultRouteDevs(defaultDevs)

	go serverStatus(it)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
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

			go func(conn net.Conn) {
				var req RequestHandler
				req.ips = it
				req.connectTimeout = viper.GetInt("connect-timeout-ms")
				req.defaultRouteDevs = defaultDevs
				req.handleRequestTransparent(conn)
			}(conn)
		}
	}()

	go func() {
		defer wg.Done()
		l, err := net.Listen("tcp", "127.0.0.1:1080")
		if err != nil {
			log.Fatalf("could not listen: %v", err)
		}

		defer l.Close()

		for {
			conn, err := l.Accept()
			if err != nil {
				log.Fatalf("accept: %v", err)
			}

			go func(conn net.Conn) {
				var req RequestHandler
				req.ips = it
				req.connectTimeout = viper.GetInt("connect-timeout-ms")
				req.defaultRouteDevs = defaultDevs
				req.handleRequestSocks4(conn)
			}(conn)
		}
	}()

	wg.Wait()
}
