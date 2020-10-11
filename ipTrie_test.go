package main

import (
	"net"
	"testing"
)

func TestTrie(t *testing.T) {
	it := makeIpTrie("", 0)

	it.insertIPv4(net.IPv4(192, 168, 178, 1), "eth0")
	it.insertIPv4(net.IPv4(192, 168, 178, 2), "eth0")
	it.insertIPv4(net.IPv4(192, 168, 178, 3), "eth0")
	it.insertIPv4(net.IPv4(192, 168, 178, 4), "eth0")
	it.insertIPv4(net.IPv4(192, 168, 178, 5), "eth0")
	it.insertIPv4Fix(net.IPv4(10, 17, 0, 1), "tener")
	it.insertIPv4(net.IPv4(172, 17, 0, 1), "tap0")

	//it.insertHostFix("heise.de", "heise-device")

	ips := []struct {
		ip  net.IP
		dev string
	}{
		{net.IPv4(172, 17, 0, 1), "tap0"},
		{net.IPv4(10, 17, 0, 1), "tener"},
		{net.IPv4(10, 17, 0, 5), "tener"},
		{net.IPv4(10, 20, 0, 1), "tener"},
		{net.IPv4(192, 188, 178, 3), "eth0"},
	}

	for _, ip := range ips {
		//fmt.Printf("ip: %+v device: %s\n", ip.ip.To4(), it.device(ip.ip))
		if it.device(ip.ip) != ip.dev {
			t.Fatalf("ip %+v has device %s but should have %s", ip.ip.To4(), it.device(ip.ip), ip.dev)
		}
	}
}
