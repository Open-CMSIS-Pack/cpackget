FROM ubuntu:23.04

RUN export DEBIAN_FRONTEND=noninteractive && \
    apt-get update && \
    apt-get -y install \
        build-essential \
        curl \
        less \
        git \
        golang \
        gopls \
        delve \
        go-staticcheck

RUN go install honnef.co/go/tools/cmd/staticcheck@latest

CMD ["/bin/bash"]
