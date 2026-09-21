FROM golang:1.23-alpine AS build
WORKDIR /src/services
COPY services/go.mod ./
COPY services/ ./
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/service ./cmd/backups

FROM postgres:17-alpine
RUN addgroup -S himate \
    && adduser -S -G himate himate \
    && mkdir -p /var/lib/himate-backups/work /offsite \
    && chown -R himate:himate /var/lib/himate-backups /offsite
COPY --from=build /out/service /app/service
USER himate
ENV PORT=10000 HIMATE_BACKUP_WORK_ROOT=/var/lib/himate-backups/work
EXPOSE 10000
ENTRYPOINT ["/app/service"]
