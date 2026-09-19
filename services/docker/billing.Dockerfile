FROM golang:1.23-bookworm AS build
WORKDIR /src/services
COPY services/go.mod ./
RUN go mod download
COPY services/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/service ./cmd/billing

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/service /app/service
ENV PORT=10000
EXPOSE 10000
USER nonroot:nonroot
ENTRYPOINT ["/app/service"]
