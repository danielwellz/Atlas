FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags='-s -w' -o /out/atlas-api ./cmd/atlas-api

FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /app
COPY --from=builder /out/atlas-api /app/atlas-api

EXPOSE 8080

ENTRYPOINT ["/app/atlas-api"]
