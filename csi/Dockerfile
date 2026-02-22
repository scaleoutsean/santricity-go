# Dockerfile for SANtricity CSI Driver
FROM golang:1.25.2 AS builder

WORKDIR /app
COPY go.mod go.sum ./
# Download dependencies
RUN go mod download

# Copy source
COPY . .

# Build the binary
# Allow build args to override version/commit
# Note: $(git ...) expansion happens in the shell, but ARG defaults are literals.
# To make this work, we must run the command inside the RUN instruction.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-X 'github.com/scaleoutsean/santricity-go/csi/driver.GitCommit=$(git rev-parse HEAD 2>/dev/null || echo unknown)'" -o santricity-csi ./csi/main.go

# Final Image
FROM alpine:3.23
RUN apk add --no-cache ca-certificates multipath-tools util-linux e2fsprogs xfsprogs open-iscsi procps btrfs-progs bcachefs-tools nvme-cli
# Note: open-iscsi provides iscsiadm

# Setup minimal env for iscsi tools if needed
RUN mkdir -p /etc/iscsi

COPY --from=builder /app/santricity-csi /bin/santricity-csi

ENTRYPOINT ["/bin/santricity-csi"]
