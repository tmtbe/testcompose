FROM golang:1.18-alpine as builder
WORKDIR /app
COPY . .
RUN go mod tidy \
    && cd agent\
    && CGO_ENABLED=0 GOOS=linux go build -o agent

FROM scratch
WORKDIR /app
COPY --from=builder /app/agent .
EXPOSE 80
ENTRYPOINT ["./agent"]