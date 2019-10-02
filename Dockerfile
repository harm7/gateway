FROM golang:alpine as builder

RUN apk update && apk add --no-cache git
WORKDIR $GOPATH/github.com/harm7/gateway/

COPY . .

RUN go get -d -v
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /go/bin/gateway


FROM scratch
COPY --from=builder /go/bin/gateway /go/bin/gateway

EXPOSE 8081
ENTRYPOINT [ "/go/bin/gateway" ]