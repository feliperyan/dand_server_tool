FROM golang:alpine as builder
RUN apk update && apk add --no-cache git ca-certificates
RUN apk add --no-cache bash
# Create appuser
RUN adduser -D -g '' appuser
WORKDIR /
COPY . .
# RUN go mod download
# RUN go mod init ddserver
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-w -s" -o /go/bin/ddserver

FROM scratch
# Import from builder.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
# Copy our static executable
COPY --from=builder /go/bin/ddserver /go/bin/ddserver
# Use an unprivileged user.
USER appuser
# Run the hello binary.
ENTRYPOINT ["/go/bin/ddserver"]
