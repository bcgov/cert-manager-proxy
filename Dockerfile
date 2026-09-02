FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/cert-manager-proxy ./cmd/cert-manager-proxy

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/cert-manager-proxy /cert-manager-proxy
ENTRYPOINT ["/cert-manager-proxy"]
