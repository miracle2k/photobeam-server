FROM golang:1.15
WORKDIR /build
COPY . .
RUN GO111MODULE=on GOOS=linux go build


# alphine seems to require CGO_ENABLED=0, which we can't do due to the crypto library we depend on!
FROM debian:latest
WORKDIR /root/
RUN apt-get update && apt-get install -y ca-certificates
COPY --from=0 /build/photobeam-server /photobeam-server
ENTRYPOINT ["/photobeam-server"]
CMD ["run"]
