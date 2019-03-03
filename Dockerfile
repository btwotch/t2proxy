FROM ubuntu
RUN apt-get update && apt-get install -y golang curl git
RUN mkdir -v /t2proxy
RUN mkdir -v /go
COPY . /t2proxy/
ENV GOPATH /go
ENV PATH ${PATH}:/go/bin
RUN echo "export GOPATH=/go >> /root/.bashrc"
RUN echo "export PATH=$PATH/go/bin >> /root/.bashrc"
RUN go get golang.org/x/tools/cmd/goimports
RUN go get github.com/LiamHaworth/go-tproxy
RUN make -C /t2proxy

CMD ["/t2proxy/t2proxy"]
