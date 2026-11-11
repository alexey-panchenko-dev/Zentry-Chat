FROM golang:1.27.4-alpine AS build

WORKDIR /src
COPY api/ ./
RUN go mod download
RUN go test -mod=mod ./...
RUN CGO_ENABLED=0 GOOS=linux go build -mod=mod -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.23

RUN apk add --no-cache ca-certificates \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app
COPY --from=build --chown=app:app /out/api ./api

USER app
ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["/app/api"]
