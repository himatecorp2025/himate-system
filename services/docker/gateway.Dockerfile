FROM ghcr.io/cirruslabs/flutter:stable AS flutter-build
WORKDIR /src/frontend
COPY frontend/pubspec.yaml frontend/analysis_options.yaml ./
RUN flutter pub get
COPY frontend/ ./
RUN flutter build web --release --no-web-resources-cdn

FROM golang:1.27.1-bookworm AS go-build
WORKDIR /src/services
COPY services/go.mod ./
COPY services/ ./
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gateway ./cmd/gateway

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=go-build /out/gateway /app/gateway
COPY --from=flutter-build /src/frontend/build/web /app/web
# The public HIMATE marketing site is source-controlled beside Flutter's web
# bootstrap. Copy it explicitly after the Flutter bundle so deploys cannot
# inherit or omit stale marketing assets.
COPY frontend/web/landing.html /app/web/landing.html
COPY frontend/web/platform.html /app/web/platform.html
COPY frontend/web/modules.html /app/web/modules.html
COPY frontend/web/programs.html /app/web/programs.html
COPY frontend/web/impact.html /app/web/impact.html
COPY frontend/web/partners.html /app/web/partners.html
COPY frontend/web/contact.html /app/web/contact.html
COPY frontend/web/himate-brand-r4.css /app/web/himate-brand-r4.css
COPY frontend/web/site.js /app/web/site.js
COPY frontend/web/art /app/web/art
COPY frontend/web/brand /app/web/brand
ENV PORT=10000
ENV WEB_DIST_DIR=/app/web
EXPOSE 10000
USER nonroot:nonroot
ENTRYPOINT ["/app/gateway"]
