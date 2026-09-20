FROM golang:1.23-alpine AS build
WORKDIR /src/services
COPY services/go.mod ./
COPY services/ ./
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/service ./cmd/cms

FROM alpine:3.21
RUN addgroup -S himate && adduser -S -G himate himate
USER himate
COPY --from=build /out/service /app/service
ENV PORT=10000
EXPOSE 10000
CMD ["/app/service"]
