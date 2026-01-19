# Dockerfile for SANtricity CSI Driver
FROM golang:1.25.2 AS builder

WORKDIR /app
COPY go.mod go.sum ./
# Download dependencies
RUN go mod download

# Copy source
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o santricity-csi ./csi/main.go

# Final Image
FROM alpine:3.19
RUN apk add --no-cache ca-certificates multipath-tools util-linux e2fsprogs xfsprogs open-iscsi
# Note: open-iscsi provides iscsiadm

# Setup minimal env for iscsi tools if needed
RUN mkdir -p /etc/iscsi

COPY --from=builder /app/santricity-csi /bin/santricity-csi

ENTRYPOINT ["/bin/santricity-csi"]
