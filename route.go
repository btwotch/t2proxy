package main

import (
	"bufio"
	"io"
	"net"
	"os"
	"regexp"
	"strconv"
	"sync"
)

type defaultRouteDevices struct {
	devs          []string
	preferredDevs []string
	sync.RWMutex
}

func makeDefaultRouteDevices(preferredDevs []string) *defaultRouteDevices {
	drd := defaultRouteDevices{}

	drd.preferredDevs = preferredDevs

	drd.update()

	return &drd
}

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

func mergeDevs(devs, preferredDevs []string) []string {
	newDevs := make([]string, 0)

	devsMap := make(map[string]bool)
	for _, dev := range devs {
		devsMap[dev] = true
	}
	preferredDevsMap := make(map[string]bool)
	for _, dev := range preferredDevs {
		preferredDevsMap[dev] = true
	}

	for _, preferredDev := range preferredDevs {
		if preferredDev == "*" {
			for dev, toBeInserted := range devsMap {
				if !toBeInserted {
					continue
				}
				// check if device has lower prio then '*'
				if preferredDevsMap[dev] {
					continue
				}

				newDevs = append(newDevs, dev)
				devsMap[dev] = false
				preferredDevsMap[dev] = false
			}
		} else if devsMap[preferredDev] == true {
			devsMap[preferredDev] = false
			preferredDevsMap[preferredDev] = false
			newDevs = append(newDevs, preferredDev)
		}
	}

	return newDevs
}

func (drd *defaultRouteDevices) get() []string {
	drd.RLock()
	defer drd.RUnlock()

	devs := make([]string, len(drd.devs))

	copy(devs, drd.devs)

	return devs
}

func (drd *defaultRouteDevices) update() {
	drd.Lock()
	defer drd.Unlock()

	defaultGw := net.IPv4(0, 0, 0, 0)

	devs := make([]string, 0)

	for _, route := range routes() {
		if route.destination.Equal(defaultGw) && route.device != "lo" {
			devs = append(devs, route.device)
		}
	}

	if len(drd.preferredDevs) == 0 {
		drd.devs = devs
	} else {
		drd.devs = mergeDevs(devs, drd.preferredDevs)
	}
}
