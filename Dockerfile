FROM golang:1.18-alpine as builder
WORKDIR /app
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

# src code
COPY . .
RUN CGO_ENABLED=0 go build -o agent ./cmd/agent

FROM alpine
WORKDIR /app
COPY --from=builder /app/agent .
EXPOSE 80
ENTRYPOINT ["./agent"]