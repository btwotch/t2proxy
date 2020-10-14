FROM ubuntu:20.04

ARG USER_ID
ARG GROUP_ID

ENV DEBIAN_FRONTEND noninteractive

RUN apt-get update && \
	apt-get install -y \
	golang \
	curl \
	git \
	make \
	strace \
	gdb
RUN apt-get update && \
	apt-get install -y \
	build-essential \
	iptables \
	vim \
	tmux \
	iproute2 \
	iputils-ping \
	netcat \
	telnet
RUN apt-get update && \
	apt-get install -y \
	systemctl

RUN /bin/yes | unminimize

RUN groupadd -g ${GROUP_ID} dev
RUN useradd -ms /bin/bash dev -u ${USER_ID} -g ${GROUP_ID}

RUN mkdir -v /t2proxy
RUN mkdir -v /root/go
ENV GOPATH /root/go
ENV PATH ${PATH}:/root/go/bin
RUN echo "export GOPATH=/root/go >> /root/.bashrc"
RUN echo "export PATH=$PATH/root/go/bin >> /root/.bashrc"

#COPY . /t2proxy/
WORKDIR /t2proxy
#CMD ["/t2proxy/t2proxy"]
