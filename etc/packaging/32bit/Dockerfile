FROM arm32v7/debian:bullseye-20230725
# docker arch guide https://github.com/docker-library/official-images#architectures-other-than-amd64

RUN apt-get update
RUN apt-get install -qqy build-essential git sudo
RUN useradd build
RUN echo "build    ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers
USER build
WORKDIR /build
COPY etc/bullseye-1.sh .
RUN sudo ./bullseye-1.sh
RUN sudo apt-get install -qqy wget xz-utils
COPY etc/bullseye-2.sh .
RUN sudo ./bullseye-2.sh
RUN sudo apt-get install -qqy file
COPY ./ rdk/
WORKDIR rdk
USER root
# RUN go build -v ./web/cmd/server
