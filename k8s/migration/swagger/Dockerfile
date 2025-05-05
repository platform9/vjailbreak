FROM golang:1.20-alpine as builder

WORKDIR /app

COPY swagger-ui ./swagger-ui
COPY main.go .
COPY openapi.yaml .
RUN go build -o swagger-server main.go


FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/swagger-ui ./swagger-ui
COPY --from=builder /app/swagger-server .
COPY --from=builder /app/openapi.yaml .

EXPOSE 8080

CMD ["./swagger-server"]
