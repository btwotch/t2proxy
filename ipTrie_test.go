package main

import (
	"net"
	"sync"
	"testing"
)

func TestTrie(t *testing.T) {
	type ipDev struct {
		ip  net.IP
		dev string
	}

	it := makeIpTrie("", 0)

	var wg sync.WaitGroup

	insertIps := []ipDev{
		{net.IPv4(192, 168, 178, 1), "eth0"},
		{net.IPv4(192, 168, 178, 2), "eth0"},
		{net.IPv4(192, 168, 178, 3), "eth0"},
		{net.IPv4(192, 168, 178, 4), "eth0"},
		{net.IPv4(192, 168, 178, 5), "eth0"},
		{net.IPv4(172, 17, 0, 1), "tap0"},
	}

	insertIpsFix := []ipDev{
		{net.IPv4(10, 17, 0, 1), "tener"},
		{net.IPv4(10, 17, 0, 2), "tener2"},
	}

	wg.Add(len(insertIps))
	for _, i := range insertIps {
		go func(i ipDev) {
			it.insertIPv4(i.ip, i.dev)
			wg.Done()
		}(i)
	}
	wg.Add(len(insertIpsFix))
	for _, i := range insertIpsFix {
		it.insertIPv4Fix(i.ip, i.dev)
		wg.Done()
	}
	wg.Wait()
	//it.insertHostFix("heise.de", "heise-device")

	it.insertIPv4Fix(net.IPv4(10, 17, 0, 2), "tener2")

	ips := []ipDev{
		{net.IPv4(172, 17, 0, 1), "tap0"},
		{net.IPv4(10, 17, 0, 1), "tener"},
		{net.IPv4(10, 17, 0, 5), "tener2"},
		{net.IPv4(10, 20, 0, 1), "tener2"},
		{net.IPv4(192, 188, 178, 3), "eth0"},
	}

	for _, ip := range ips {
		if it.device(ip.ip) != ip.dev {
			t.Fatalf("ip %+v has device %s but should have %s", ip.ip.To4(), it.device(ip.ip), ip.dev)
		}
	}

	if it.deviceFix(net.IPv4(10, 17, 0, 1)) != "tener" {
		t.Fatalf("ip 10.17.0.1 has device %s but should have tener", it.deviceFix(net.IPv4(10, 17, 0, 1)))
	}

	if it.deviceFix(net.IPv4(10, 17, 0, 3)) != "" {
		t.Fatalf("ip 10.17.0.3 has device %s but should have ''", it.deviceFix(net.IPv4(10, 17, 0, 3)))
	}
}
