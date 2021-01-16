FROM golang:1.15-alpine AS builder
WORKDIR /src
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN go build main.go

FROM alpine
WORKDIR /app
COPY --from=builder /src/main .
CMD ["./main"]
