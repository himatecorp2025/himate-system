FROM ghcr.io/cirruslabs/flutter:stable AS flutter-build
WORKDIR /src/frontend
COPY frontend/pubspec.yaml frontend/analysis_options.yaml ./
RUN flutter pub get
COPY frontend/ ./
RUN flutter build web --release

FROM golang:1.23-bookworm AS go-build
WORKDIR /src/backend
COPY backend/go.mod ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/himate ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=go-build /out/himate /app/himate
COPY --from=flutter-build /src/frontend/build/web /app/web
ENV PORT=10000
ENV WEB_DIST_DIR=/app/web
ENV HIMATE_ENV=production
ENV COOKIE_SECURE=true
EXPOSE 10000
USER nonroot:nonroot
ENTRYPOINT ["/app/himate"]
