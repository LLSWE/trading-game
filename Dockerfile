FROM golang:1.27-alpine AS builder

WORKDIR /app


RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/server ./cmd/server/main.go


FROM alpine:3.19

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/bin/server /app/server

ENTRYPOINT ["/app/server"]
