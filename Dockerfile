FROM golang:1.25-alpine AS builder

WORKDIR /blockemulator

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/consensusnode cmd/consensusnode/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/supervisor cmd/supervisor/main.go

FROM alpine:3 AS consensusnode

COPY --from=builder /bin/consensusnode /blockemulator/config.yaml /blockemulator/ip_table.json /

ENTRYPOINT ["/consensusnode"]
CMD ["-h"]


FROM alpine:3 AS supervisor

COPY --from=builder /bin/supervisor /blockemulator/config.yaml /blockemulator/ip_table.json /

ENTRYPOINT ["/supervisor"]
CMD ["-h"]
