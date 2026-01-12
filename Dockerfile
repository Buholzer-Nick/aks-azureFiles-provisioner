FROM golang:1.25 AS builder

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/azurefile-provisioner ./cmd/manager

FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /
COPY --from=builder /out/azurefile-provisioner /azurefile-provisioner

EXPOSE 8080 8081
USER nonroot:nonroot

ENTRYPOINT ["/azurefile-provisioner"]
