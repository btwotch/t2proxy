#!/bin/bash

if [ $# -eq 0 ]
then
        echo "No arguments supplied"
        exit 1
fi

set -euo pipefail

iptables -F TTWO -t nat
iptables -X TTWO -t nat

ip6tables -F TTWO -t nat
ip6tables -X TTWO -t nat

iptables -D OUTPUT -p tcp -m owner --uid-owner $1 -j TTWO -t nat
