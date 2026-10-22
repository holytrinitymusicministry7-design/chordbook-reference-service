
FROM golang:1.24-alpine AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /reference-service ./server.go

FROM alpine:3.22
RUN adduser -D -H appuser
WORKDIR /app

COPY --from=build /reference-service /app/reference-service

USER appuser
EXPOSE 8080

CMD ["/app/reference-service"]
