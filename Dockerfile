FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

# Cross-compiles for the target platform (multi-arch images without emulating the Go build).
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/kyber ./cmd/kyber && mkdir -p /out/data/attachments

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/kyber /kyber
# Uploaded files (mount a volume here); owned by the runtime user so a fresh volume is writable.
COPY --from=build --chown=nonroot:nonroot /out/data /data
ENV KYBER_ATTACHMENTS_DIR=/data/attachments
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/kyber"]
