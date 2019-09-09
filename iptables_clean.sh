#!/bin/sh

iptables -F REDSOCKS -t nat
iptables -X REDSOCKS -t nat

ip6tables -F REDSOCKS -t nat
ip6tables -X REDSOCKS -t nat

#iptables -P INPUT ACCEPT
#iptables -P FORWARD ACCEPT
#iptables -P OUTPUT ACCEPT
#iptables -t nat -F
#iptables -t mangle -F
#iptables -F
#iptables -X
