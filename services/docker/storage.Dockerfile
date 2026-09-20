FROM golang:1.23-alpine AS build
WORKDIR /src/services
COPY services/go.mod services/go.sum ./
RUN go mod download
COPY services ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/service ./cmd/storage

FROM alpine:3.21
RUN addgroup -S himate && adduser -S -G himate himate && mkdir -p /data/partners && chown -R himate:himate /data
USER himate
COPY --from=build /out/service /app/service
ENV PORT=10000 HIMATE_STORAGE_ROOT=/data/partners
EXPOSE 10000
CMD ["/app/service"]
