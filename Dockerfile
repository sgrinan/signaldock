# Build

FROM --platform=$BUILDPLATFORM golang:1.27-bookworm AS build

WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH

ARG VERSION=dev
ARG COMMIT=unknown

RUN CGO_ENABLED=0 \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH \
    go build \
        -trimpath \
        -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT}" \
        -o /out/signaldock \
        ./cmd/signaldock


        
# Runtime

FROM gcr.io/distroless/static-debian13:nonroot

WORKDIR /app

COPY --from=build /out/signaldock /app/signaldock

EXPOSE 8080

ENTRYPOINT ["/app/signaldock"]