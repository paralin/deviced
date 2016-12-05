FROM alpine:edge
RUN apk --update add ca-certificates
ADD dumb-init /
ADD deviced /
RUN chmod +x /deviced /dumb-init
ENTRYPOINT ["/dumb-init", "/deviced"]
