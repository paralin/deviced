FROM alpine:edge
RUN apk --update add ca-certificates
ADD deviced /
ENTRYPOINT ["/deviced"]
