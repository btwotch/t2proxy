package main

import (
	"bufio"
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"
)

func TestProxy(t *testing.T) {
	uu, _ := url.Parse("http://heise.de/foo.html")
	//uu, _ := u.Parse("/foo.html")

	req, _ := http.NewRequest("GET", uu.String(), nil)

	remoteConn, _ := net.Dial("tcp", "localhost:3128")

	req.WriteProxy(remoteConn)
	req.WriteProxy(os.Stdout)

	srcReader := bufio.NewReader(remoteConn)
	res, err := http.ReadResponse(srcReader, req)
	if err != nil {
		t.Errorf("err: %v\n", err)
	}
	//t.Errorf("req: %v\n", req.URL.String())
	//t.Errorf("res: %v\n", res)
	if res.StatusCode != 301 {
		t.Errorf("res: %v\n", res)
	}
}
