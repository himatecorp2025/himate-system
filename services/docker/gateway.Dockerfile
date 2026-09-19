FROM ghcr.io/cirruslabs/flutter:stable AS flutter-build
WORKDIR /src/frontend
COPY frontend/pubspec.yaml frontend/analysis_options.yaml ./
RUN flutter pub get
COPY frontend/ ./
RUN flutter build web --release

FROM golang:1.23-bookworm AS gold
WORKDIR /src/fronservices
COPY services/go.mod ./
RUN go mod download
COPY services/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gateway ./cmd/gateway

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=gold
WOR /out/gateway /app/gateway
COPY --from=flutter-build /src/frontend/build/web /app/web
ENV PORT=10000
ENV WEB_DIST_DIR=/app/web
EXPOSE 10000
USER nonroot:nonroot
ENTRYPOINT ["/app/gateway"]
