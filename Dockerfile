FROM alpine:latest

COPY alertmanager-zabbix-provisioner /usr/bin
RUN chmod +x /usr/bin/alertmanager-zabbix-provisioner

RUN mkdir -p /etc/provisioner
COPY config.yaml /etc/provisioner

RUN adduser provisioner -s /bin/false -D provisioner
USER provisioner

CMD ["/usr/bin/alertmanager-zabbix-provisioner", "-config", "/etc/provisioner/config.yaml"]
