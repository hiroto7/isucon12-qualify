#!/bin/bash
set -eu
peer=$1
iptables -N ISUCON_OUTPUT 2>/dev/null || iptables -F ISUCON_OUTPUT
iptables -C OUTPUT -j ISUCON_OUTPUT 2>/dev/null || iptables -I OUTPUT 1 -j ISUCON_OUTPUT
iptables -A ISUCON_OUTPUT -o lo -j ACCEPT
iptables -A ISUCON_OUTPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
iptables -A ISUCON_OUTPUT -d "$peer" -j ACCEPT
iptables -A ISUCON_OUTPUT -p udp --dport 53 -j ACCEPT
iptables -A ISUCON_OUTPUT -p udp --dport 67 -j ACCEPT
for subnet in 10.0.0.0/8 172.16.0.0/12 192.168.0.0/16 169.254.0.0/16; do
  iptables -A ISUCON_OUTPUT -d "$subnet" -j REJECT
done
sysctl -w net.ipv6.conf.all.disable_ipv6=1
sysctl -w net.ipv6.conf.default.disable_ipv6=1
