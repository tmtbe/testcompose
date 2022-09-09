FROM golang:1.18-alpine as builder
WORKDIR /app
COPY . .
RUN go mod tidy \
    && CGO_ENABLED=0 go build -o agent ./cmd/agent

FROM alpine
WORKDIR /app
COPY --from=builder /app/agent .
EXPOSE 80
ENTRYPOINT ["./agent"]