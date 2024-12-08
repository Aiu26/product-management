FROM golang:latest
WORKDIR /common
COPY ./common/go.* .
RUN go mod download
WORKDIR /app
COPY ./products/go.* .
RUN go mod download
WORKDIR /common
COPY ./common .
WORKDIR /app
COPY ./products .
RUN go build -o main cmd/main.go
CMD ["./main"]
