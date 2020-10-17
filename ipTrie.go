package main

import (
	"fmt"
	"log"
	"net"
	"sync"
)

type IpTrie struct {
	dev  string
	its  map[byte]*IpTrie
	name byte
	sync.RWMutex
	redirectIp *net.IP
}

func makeIpTrie(device string, name byte) *IpTrie {
	var ret *IpTrie

	ret = &IpTrie{}
	ret.dev = device
	ret.name = name

	ret.its = make(map[byte]*IpTrie)

	return ret
}

func (it *IpTrie) insertChild(val byte, device string) *IpTrie {
	var ret *IpTrie

	it.Lock()
	defer it.Unlock()

	ret, ok := it.its[val]
	if !ok {
		ret = makeIpTrie(device, val)
		it.its[val] = ret
	} else {
		ret.Lock()
		ret.dev = device
		ret.Unlock()
	}

	return ret
}

func (it *IpTrie) findIPv4(val net.IP) (*IpTrie, int) {
	bytes := val.To4()

	depth := 0

	i := it
	for _, b := range bytes {
		i.Lock()
		defer i.Unlock()

		c, ok := i.its[b]
		if ok {
			i = c
		} else {
			return i, depth
		}

		depth++
	}

	return i, depth
}

func (it *IpTrie) device(val net.IP) string {
	i, _ := it.findIPv4(val)

	return i.dev
}

func (it *IpTrie) deviceFix(val net.IP) string {
	i, depth := it.findIPv4(val)

	if depth == len(val.To4()) {
		return i.dev
	}

	return ""
}

func (it *IpTrie) insertIPv4Fix(val net.IP, device string, redirect *net.IP) {
	bytes := val.To4()

	c := it

	for i := 0; i < len(bytes); i++ {
		c = c.insertChild(bytes[i], device)
	}

	c.redirectIp = redirect
}

func (it *IpTrie) insertIPv4(val net.IP, device string) {
	bytes := val.To4()

	i, depth := it.findIPv4(val)
	if depth == len(bytes) {
		return
	}

	i.insertChild(bytes[depth], device)
}

func (it *IpTrie) insertFix(address string, device string, redirect *net.IP) {
	addrs, err := net.LookupIP(address)
	if err != nil {
		log.Printf("Looking up host %s failed: %v", address, err)
		return
	}

	for _, addr := range addrs {
		if addr.To4() == nil {
			continue
		}
		it.insertIPv4Fix(addr, device, redirect)
	}
}

func (it *IpTrie) insertHostFix(address string, device string) {
	it.insertFix(address, device, nil)
}

func (it *IpTrie) insertRedirectFix(address string, redirect net.IP) {
	it.insertFix(address, "", &redirect)
}

func (it *IpTrie) redirect() net.IP {
	it.RLock()
	defer it.RUnlock()

	return *it.redirectIp
}

func (it *IpTrie) redirectFix(val net.IP) net.IP {
	i, depth := it.findIPv4(val)

	if depth == len(val.To4()) {
		i.RLock()
		defer i.RUnlock()
		if i.redirectIp != nil {
			return *i.redirectIp
		}
	}

	return val
}

func (it *IpTrie) _dump(pre string, ret *string, depth int) {
	it.Lock()
	defer it.Unlock()

	for k, v := range it.its {
		var newPre string
		if pre == "" {
			newPre = fmt.Sprintf("%d", k)
		} else {
			newPre = fmt.Sprintf("%s.%d", pre, k)
		}
		v._dump(newPre, ret, depth+1)
	}

	if len(it.its) == 0 {
		redirectIp := ""
		if it.redirectIp != nil {
			redirectIp = it.redirectIp.String()
		}
		*ret = fmt.Sprintf("%s%s -> %s on dev %s\n", *ret, pre, redirectIp, it.dev)
	}
}

func (it *IpTrie) dump() string {
	var ret string

	it._dump("", &ret, 0)

	return ret
}

func (it *IpTrie) dumpTrieElemChildren() {
	fmt.Printf("%d(%p) %s - children: ", it.name, it, it.dev)
	for k, _ := range it.its {
		fmt.Printf("%d ", k)
	}

	fmt.Println("")
}
