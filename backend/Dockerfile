FROM golang:1.27.3-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -o /crawler ./cmd/crawler
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /api /api
COPY --from=build /crawler /crawler
USER 65532:65532
ENV LISTEN_HOST=0.0.0.0
EXPOSE 8080
ENTRYPOINT ["/api"]
