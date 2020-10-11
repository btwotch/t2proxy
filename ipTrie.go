package main

import (
	"fmt"
	"log"
	"net"
)

type IpTrie struct {
	dev  string
	its  map[byte]*IpTrie
	name byte
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

	ret, ok := it.its[val]
	if !ok {
		ret = makeIpTrie(device, val)
		it.its[val] = ret
	}

	return ret
}

func (it *IpTrie) findIPv4(val net.IP) (*IpTrie, int) {
	bytes := val.To4()

	depth := 0

	i := it
	for _, b := range bytes {
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

func (it *IpTrie) insertIPv4Fix(val net.IP, device string) {
	bytes := val.To4()

	c := it

	for i := 0; i < len(bytes); i++ {
		c = c.insertChild(bytes[i], device)
	}
}

func (it *IpTrie) insertIPv4(val net.IP, device string) {
	bytes := val.To4()

	i, depth := it.findIPv4(val)
	if depth == len(bytes) {
		return
	}

	i.insertChild(bytes[depth], device)
}

func (it *IpTrie) insertHostFix(address string, device string) {
	addrs, err := net.LookupIP(address)
	if err != nil {
		log.Printf("Looking up host %s failed: %v", address, err)
		return
	}

	for _, addr := range addrs {
		if addr.To4() == nil {
			continue
		}
		it.insertIPv4Fix(addr, device)
	}
}

func (it *IpTrie) dump(pre string, depth int) {
	for k, v := range it.its {
		newPre := fmt.Sprintf("%s.%d", pre, k)
		v.dump(newPre, depth+1)
	}

	if len(it.its) == 0 {
		fmt.Printf("%s -> %s\n", pre, it.dev)
	}
}

func (it *IpTrie) dumpTrieElemChildren() {
	fmt.Printf("%d(%p) %s - children: ", it.name, it, it.dev)
	for k, _ := range it.its {
		fmt.Printf("%d ", k)
	}

	fmt.Println("")
}
