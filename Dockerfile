FROM golang:1.24 AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/api ./cmd/api

FROM gcr.io/distroless/base-debian12
WORKDIR /app

COPY --from=builder /bin/api /app/api
COPY migrations /app/migrations
COPY swagger /app/swagger

ENV HTTP_ADDR=:8080
EXPOSE 8080

USER nonroot:nonroot
ENTRYPOINT ["/app/api"]
