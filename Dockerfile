FROM golang:alpine
COPY . /go/src/github.com/iavael/docker-ovs-plugin
WORKDIR /go/src/github.com/iavael/docker-ovs-plugin
RUN apk add --no-cache -t docker-ovs-plugin godep git build-base linux-headers && go get -v && apk del docker-ovs-plugin
VOLUME /var/run/docker.sock:/var/run/docker.sock
VOLUME /run/docker/plugins/:/run/docker/plugins/
ENTRYPOINT ["docker-ovs-plugin"]
