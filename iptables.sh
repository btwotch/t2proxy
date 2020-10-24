#!/bin/bash

if [ $# -eq 0 ]
then
	echo "No arguments supplied"
	exit 1
fi

iptables -t nat -N TTWO
ip6tables -t nat -N TTWO

#set -euo pipefail

iptables -t nat -A TTWO -d 0.0.0.0/8 -j RETURN
iptables -t nat -A TTWO -d 10.0.0.0/8 -j RETURN
iptables -t nat -A TTWO -d 127.0.0.0/8 -j RETURN
iptables -t nat -A TTWO -d 169.254.0.0/16 -j RETURN
iptables -t nat -A TTWO -d 172.16.0.0/12 -j RETURN
iptables -t nat -A TTWO -d 192.168.0.0/16 -j RETURN
iptables -t nat -A TTWO -d 224.0.0.0/4 -j RETURN
iptables -t nat -A TTWO -d 240.0.0.0/4 -j RETURN

# Anything else should be redirected to port 12345
iptables -t nat -A TTWO -p tcp -j REDIRECT --to-ports 3129
iptables -t nat -A TTWO -p udp -j REDIRECT --to-ports 3129

ip6tables -t nat -A TTWO -p tcp -j REDIRECT --to-ports 3129
ip6tables -t nat -A TTWO -p udp -j REDIRECT --to-ports 3129
# Any tcp connection made by `luser' should be redirected.
iptables -t nat -A OUTPUT -p tcp -m owner --uid-owner $1 -j TTWO
iptables -t nat -A OUTPUT -p udp -m owner --uid-owner $1 -j TTWO
ip6tables -t nat -A OUTPUT -p tcp -m owner --uid-owner $1 -j TTWO
ip6tables -t nat -A OUTPUT -p udp -m owner --uid-owner $1 -j TTWO
