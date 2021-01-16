FROM golang:alpine AS builder
WORKDIR /src
COPY ./ ./
RUN go mod download
RUN go build main.go

FROM alpine
WORKDIR /app
COPY --from=builder /src/main .
CMD ["./main"]
