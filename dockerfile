FROM golang:1.25-alpine3.21

RUN mkdir /app

ADD . /app 

WORKDIR /app

COPY . ./


RUN go mod download 

RUN go build -o main ./cmd/server/main.go

CMD ["/app/main"] 