FROM alpine:3.3
RUN apk --update add ca-certificates
ADD deviced /
CMD ["/deviced"]
