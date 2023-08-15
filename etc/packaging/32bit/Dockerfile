FROM arm32v7/ubuntu:jammy-20230624
# docker arch guide https://github.com/docker-library/official-images#architectures-other-than-amd64

RUN apt update
RUN apt install -qqy build-essential
RUN apt install -qqy git
WORKDIR /build
RUN git clone https://github.com/viamrobotics/rdk.git
WORKDIR rdk
RUN etc/setup.sh
RUN make server
