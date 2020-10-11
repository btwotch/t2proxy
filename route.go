package main

import (
	"bufio"
	"io"
	"net"
	"os"
	"regexp"
	"strconv"
)

func parseHexIPv4(hex string) [4]byte {
	var ret [4]byte

	for i, k := range []int{6, 4, 2, 0} {
		p, err := strconv.ParseInt(hex[k:k+2], 16, 16)
		if err != nil {
			panic(err)
		}
		ret[i] = byte(p)
	}

	return ret
}

type Route struct {
	device      string
	destination net.IP
	gateway     net.IP
}

func routes() []Route {
	var routes []Route

	routes = make([]Route, 0)

	file, err := os.Open("/proc/net/route")
	if err != nil {
		panic(err)
	}

	defer file.Close()
	r := bufio.NewReader(file)

	// skip first line
	_, err = r.ReadString('\n')
	if err != nil {
		panic(err)
	}

	rex := regexp.MustCompile(`(?P<device>\S+)\s+(?P<destination>[0-9A-Za-z]+)\s+(?P<gateway>[0-9A-Za-z]+)`)
	for {
		var route Route
		line, err := r.ReadString('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}

		rexMatch := rex.FindStringSubmatch(line)
		for i, name := range rex.SubexpNames() {
			if i == 0 {
				continue
			}
			switch name {
			case "":
				continue
			case "destination":
				ipAddr := parseHexIPv4(rexMatch[i])
				route.destination = net.IPv4(ipAddr[0], ipAddr[1], ipAddr[2], ipAddr[3])
			case "gateway":
				ipAddr := parseHexIPv4(rexMatch[i])
				route.gateway = net.IPv4(ipAddr[0], ipAddr[1], ipAddr[2], ipAddr[3])
			default:
				route.device = rexMatch[i]
			}
		}

		routes = append(routes, route)
	}

	return routes
}

func defaultRouteDevices() []string {
	devs := make([]string, 0)

	defaultGw := net.IPv4(0, 0, 0, 0)

	for _, route := range routes() {
		if route.destination.Equal(defaultGw) {
			devs = append(devs, route.device)
		}
	}

	return devs
}
