package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/LiamHaworth/go-tproxy"
)

func handleTCPConn(conn net.Conn) {
	log.Printf("Accepting TCP connection from %s with destination of %s", conn.RemoteAddr().String(), conn.LocalAddr().String())

	remoteConn, err := conn.(*tproxy.Conn).DialOriginalDestination(false)
	if err != nil {
		log.Printf("Failed to connect to original destination [%s]: %s", conn.LocalAddr().String(), err)
	} else {
		defer remoteConn.Close()
		defer conn.Close()
		return
	}

	var streamWait sync.WaitGroup
	streamWait.Add(2)

	streamConn := func(dst io.Writer, src io.Reader) {
		io.Copy(dst, src)
		streamWait.Done()
	}

	go streamConn(remoteConn, conn)
	go streamConn(conn, remoteConn)

	streamWait.Wait()
}

func main() {
	var err error

	fmt.Println("t2proxy")

	tcpListener, err := tproxy.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 3128})
	if err != nil {
		log.Fatalf("ListenTCP failed: %v", err)
	}

	for {
		conn, err := tcpListener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
				log.Printf("Temporary error while accepting connection: %s", netErr)
			}

			log.Fatalf("Unrecoverable error while accepting connection: %s", err)
			return
		}

		fmt.Printf("conn: %s -> %s\n", conn.LocalAddr().String(), conn.RemoteAddr().String())
		go handleTCPConn(conn)

	}
}
