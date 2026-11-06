FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/fileshare ./cmd/fileshare

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
  && adduser -D -H -u 1000 app
WORKDIR /app
COPY --from=build /out/fileshare /app/fileshare
RUN mkdir -p /data && chown app:app /data
USER app
ENV DATA_DIR=/data
ENV LISTEN_ADDR=:8080
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/app/fileshare"]
