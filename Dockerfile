FROM golang:1.11.2-alpine3.8 AS builder
WORKDIR /go/src/github.com/ovc-disk-csi-driver
ADD . .
RUN apk add --no-cache make && make


FROM alpine:3.8
RUN apk add --no-cache ca-certificates e2fsprogs
COPY --from=builder /go/src/github.com/ovc-disk-csi-driver/bin/ovc-csi-driver /bin/ovc-disk-csi-driver
ENTRYPOINT ["/bin/ovc-disk-csi-driver"]