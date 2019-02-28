# Build
FROM golang:1 as build

WORKDIR /go/src/github.com/gmauleon/alertmanager-zabbix-provisioner
ADD . .

RUN go get -d -t -v ./...
RUN go test -v ./...
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o alertmanager-zabbix-provisioner .

# Run
FROM alpine:latest

# add actual CA certificates
RUN apk add --update ca-certificates && update-ca-certificates

RUN adduser provisioner -s /bin/false -D provisioner

RUN mkdir -p /etc/provisioner
COPY config.yaml /etc/provisioner

COPY --from=build /go/src/github.com/gmauleon/alertmanager-zabbix-provisioner/alertmanager-zabbix-provisioner /usr/bin

USER provisioner

ENTRYPOINT ["/usr/bin/alertmanager-zabbix-provisioner"]
CMD ["-config", "/etc/provisioner/config.yaml"]
