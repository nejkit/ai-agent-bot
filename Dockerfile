FROM golang:1.23.4-alpine as builder
WORKDIR /build
COPY go.mod .
RUN go mod download
COPY . .
RUN go build -o /main main.go

FROM alpine:3
RUN apk add --no-cache tzdata
ENV TZ=Europe/Kyiv
RUN ln -sf /usr/share/zoneinfo/Europe/Kyiv /etc/localtime && echo "Europe/Kyiv" > /etc/timezone

COPY --from=builder main /bin/main
ENTRYPOINT [ "/bin/main" ]