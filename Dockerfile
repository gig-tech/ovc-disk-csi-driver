FROM golang:1.11.13-alpine3.10 AS builder
WORKDIR /tmp
ADD . .
RUN apk add --no-cache make
RUN make


FROM alpine:3.10
RUN apk add --no-cache ca-certificates e2fsprogs
COPY --from=builder /tmp/bin/ovc-csi-driver /bin/ovc-disk-csi-driver
ENTRYPOINT ["/bin/ovc-disk-csi-driver"]
