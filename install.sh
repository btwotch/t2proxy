#!/bin/bash

#if [ $# -eq 0 ]
#then
#	echo "No arguments supplied"
#	exit 1
#fi
#
set -euo pipefail

useradd -d /dev/null -s /bin/false t2proxy
cp -v t2proxy /usr/bin/t2proxy
cp -v t2proxy.service /lib/systemd/system
cp -v iptables.sh /usr/bin/t2proxy_iptables.sh
cp -v iptables_clean.sh /usr/bin/t2proxy_iptables_clean.sh
install t2proxy.yaml /etc -o t2proxy
