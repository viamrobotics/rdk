FROM debian:bullseye

RUN apt-get update && apt-get install sudo
RUN adduser --disabled-password runner
RUN echo '%sudo ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers
RUN adduser runner sudo
USER runner
WORKDIR /build
COPY etc/setup.sh .
RUN ./setup.sh
RUN sudo apt-get install -qqy wget xz-utils file vim pkg-config
WORKDIR /rdk
