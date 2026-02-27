FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o navigatorr .

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/navigatorr /usr/local/bin/navigatorr
ENTRYPOINT ["navigatorr"]
