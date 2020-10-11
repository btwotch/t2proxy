#!/bin/sh

iptables -F TTWO -t nat
iptables -X TTWO -t nat

ip6tables -F TTWO -t nat
ip6tables -X TTWO -t nat

#iptables -P INPUT ACCEPT
#iptables -P FORWARD ACCEPT
#iptables -P OUTPUT ACCEPT
#iptables -t nat -F
#iptables -t mangle -F
#iptables -F
#iptables -X
