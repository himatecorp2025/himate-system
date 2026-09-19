FROM ghcr.io/cirruslabs/flutter:stable AS flutter-build
WORKDIR /src/frontend
COPY frontend/pubspec.yaml frontend/analysis_options.yaml ./
RUN flutter pub get
COPY frontend/ ./
RUN flutter build web --release

FROM golang:1.23-bookworm AS go-build
WORKDIR /src/services
COPY services/go.mod ./
COPY services/ ./
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gateway ./cmd/gateway

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=go-build /out/gateway /app/gateway
COPY --from=flutter-build /src/frontend/build/web /app/web
ENV PORT=10000
ENV WEB_DIST_DIR=/app/web
EXPOSE 10000
USER nonroot:nonroot
ENTRYPOINT ["/app/gateway"]
