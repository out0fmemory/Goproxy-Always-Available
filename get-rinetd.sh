#!/bin/bash
# see https://github.com/linhua55/lkl_study

set -xe

export RINET_URL="https://drive.google.com/uc?id=0B0D0hDHteoksVW5CemJKZVcyN1E"

if [ "$(id -u)" != "0" ]; then
    echo "ERROR: Please run as root"
    exit 1
fi

for CMD in curl iptables grep cut xargs systemctl
do
	if ! type -p ${CMD}; then
		echo -e "\e[1;31mtool ${CMD} is not installed, abort.\e[0m"
		exit 1
	fi
done

curl -L "${RINET_URL}" >/usr/bin/rinetd-bbr
chmod +x /usr/bin/rinetd-bbr

cat <<EOF > /etc/rinetd-bbr.conf
# bindadress bindport connectaddress connectport
0.0.0.0 443 0.0.0.0 443
0.0.0.0 80 0.0.0.0 80
EOF

cat <<EOF > /etc/systemd/system/rinetd-bbr.service
[Unit]
Description=rinetd with brr

[Service]
ExecStart=/usr/bin/rinetd-bbr -f -c /etc/rinetd-bbr.conf raw venet0:0
Restart=always

[Install]
WantedBy=multi-user.target
EOF

systemctl enable rinetd-bbr.service

systemctl start rinetd-bbr.service

