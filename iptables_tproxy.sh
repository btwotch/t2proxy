#!/bin/bash

#iptables -t mangle -A PREROUTING ! -d 127.0.0.1 -p tcp -j TPROXY --on-port 3129 --on-ip 127.0.0.1 --tproxy-mark 0x1/0x1
#ip rule add fwmark 1 lookup 100
#ip route add local 0.0.0.0/0 dev lo table 100

#ip r del default
#ip r a default via 127.0.0.1


echo 0 > /proc/sys/net/ipv4/conf/tap0/rp_filter
echo 1 > /proc/sys/net/ipv4/ip_forward
echo 0 > /proc/sys/net/ipv4/conf/default/rp_filter
echo 0 > /proc/sys/net/ipv4/conf/all/rp_filter

iptables -t mangle -N DIVERT
iptables -t mangle -A PREROUTING -p tcp -m socket -j DIVERT
iptables -t mangle -A DIVERT -j MARK --set-mark 1
iptables -t mangle -A DIVERT -j ACCEPT

ip rule add fwmark 1 lookup 32765
ip route add local 0.0.0.0/0 dev lo table 100

iptables -t mangle -A PREROUTING -p tcp --dport 80 -j TPROXY --tproxy-mark 0x1/0x1 --on-port 3129 --on-ip 127.0.0.1

ip r del default
ip r a default via 127.0.0.1
