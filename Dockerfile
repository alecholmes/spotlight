# https://github.com/hadv/eb-echo-docker

FROM golang:1.7

# Set GOPATH/GOROOT environment variables
RUN mkdir -p /go
ENV GOPATH /go
ENV PATH $GOPATH/bin:$PATH

# go get all of the dependencies
RUN go get github.com/tools/godep

# Set up app
ADD . /go/src/github.com/alecholmes/spotlight
WORKDIR /go/src/github.com/alecholmes/spotlight
RUN godep go build -v

# 3000
EXPOSE 8989

CMD ["go", "run", "main.go", "-stderrthreshold=INFO"]
