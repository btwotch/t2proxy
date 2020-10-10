#!/bin/bash

if [ $# -eq 0 ]
then
	echo "No arguments supplied"
	exit 1
fi

iptables -t nat -N REDSOCKS
ip6tables -t nat -N REDSOCKS

set -euo pipefail

iptables -t nat -A REDSOCKS -d 0.0.0.0/8 -j RETURN
iptables -t nat -A REDSOCKS -d 10.0.0.0/8 -j RETURN
iptables -t nat -A REDSOCKS -d 127.0.0.0/8 -j RETURN
iptables -t nat -A REDSOCKS -d 169.254.0.0/16 -j RETURN
iptables -t nat -A REDSOCKS -d 172.16.0.0/12 -j RETURN
iptables -t nat -A REDSOCKS -d 192.168.0.0/16 -j RETURN
iptables -t nat -A REDSOCKS -d 224.0.0.0/4 -j RETURN
iptables -t nat -A REDSOCKS -d 240.0.0.0/4 -j RETURN

# Anything else should be redirected to port 12345
iptables -t nat -A REDSOCKS -p tcp -j REDIRECT --to-ports 3129

ip6tables -t nat -A REDSOCKS -p tcp -j REDIRECT --to-ports 3129
# Any tcp connection made by `luser' should be redirected.
iptables -t nat -A OUTPUT -p tcp -m owner --uid-owner $1 -j REDSOCKS
ip6tables -t nat -A OUTPUT -p tcp -m owner --uid-owner $1 -j REDSOCKS
