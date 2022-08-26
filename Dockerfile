FROM golang AS builder

WORKDIR /root

# Dependencies
RUN go install github.com/koron/go-ssdp@latest && \
	go install github.com/gorilla/websocket@latest && \
	go install github.com/kardianos/osext@latest


# Copy of source files
COPY . .

# Disable CGO Tool
ENV CGO_ENABLED=0

# xTeVe build
RUN go build xteve.go

#                    #
# xTeVe docker image #
#                    #
FROM alpine:latest  

CMD ["/usr/local/bin/xteve", "-config", "/data"] 

ENV UID=1000
ENV GID=1000

EXPOSE 34400   

# User creation and installation of ca-certificates, ffmpeg and vlc
RUN addgroup -g $UID -S xteve  && \
	adduser -u $GID -S xteve -G xteve && \
	mkdir /data && \
	chown $UID:$GID /data && \
	apk add --no-cache ca-certificates curl ffmpeg vlc && \
	rm -rf /var/cache/apk/*

# Copy binary from build stage
COPY --from=builder /root/xteve /usr/local/bin/

USER xteve

HEALTHCHECK --interval=1m --timeout=3s \
  CMD curl --fail http://localhost:34400/web || exit 1

VOLUME ["/data"]
