FROM alpine:edge
RUN apk --update add ca-certificates
ADD dumb-init /
ADD deviced /
ENTRYPOINT ["/dumb-init", "/deviced"]
