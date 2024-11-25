FROM golang:1.22 AS builder
WORKDIR /app
COPY . .
RUN go build -o main .
FROM gcr.io/distroless/base-debian12
WORKDIR /
COPY --from=builder /app/main .
CMD ["./main"]
