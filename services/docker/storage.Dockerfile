FROM golang:1.27.1-bookworm AS build
WORKDIR /src/services
COPY services/go.mod ./
COPY services/ ./
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/service ./cmd/storage

FROM alpine:3.21
RUN apk add --no-cache su-exec \
    && addgroup -S himate \
    && adduser -S -G himate himate \
    && mkdir -p /data/partners \
    && chown -R himate:himate /data
COPY --from=build /out/service /app/service
ENV PORT=10000 HIMATE_STORAGE_ROOT=/data/partners
EXPOSE 10000
ENTRYPOINT ["sh","-c","mkdir -p \"$HIMATE_STORAGE_ROOT\" && chown -R himate:himate /data && exec su-exec himate /app/service"]
