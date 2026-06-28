FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/atlas-api ./cmd/atlas-api

FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /app
COPY --from=builder /out/atlas-api /app/atlas-api

EXPOSE 8080

ENTRYPOINT ["/app/atlas-api"]
