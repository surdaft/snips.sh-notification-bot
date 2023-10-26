FROM golang:1.21 AS build

WORKDIR /app
COPY . .

RUN go get -u
RUN go build -o binary .

ENTRYPOINT /app/binary