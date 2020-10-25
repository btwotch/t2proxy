#!/bin/bash

echo 0 > /proc/sys/net/ipv4/conf/tap0/rp_filter
echo 1 > /proc/sys/net/ipv4/ip_forward
echo 0 > /proc/sys/net/ipv4/conf/default/rp_filter
echo 0 > /proc/sys/net/ipv4/conf/all/rp_filter

iptables -t mangle -N DIVERT
iptables -t mangle -A PREROUTING -m socket -j DIVERT
iptables -t mangle -A DIVERT -j MARK --set-mark 1
iptables -t mangle -A DIVERT -j ACCEPT

ip rule add fwmark 1 lookup 32765
ip route add local 0.0.0.0/0 dev lo table 100

iptables -t mangle -A PREROUTING -p tcp -j TPROXY --tproxy-mark 0x1/0x1 --on-port 3129 --on-ip 127.0.0.1
iptables -t mangle -A PREROUTING -p udp -j TPROXY --tproxy-mark 0x1/0x1 --on-port 3129 --on-ip 127.0.0.1

ip r a default via 127.0.0.1 dev lo metric 100
