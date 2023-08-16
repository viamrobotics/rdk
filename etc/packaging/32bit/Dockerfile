FROM arm32v7/debian:bullseye-20230725
# docker arch guide https://github.com/docker-library/official-images#architectures-other-than-amd64

WORKDIR /build
COPY etc/bullseye-1.sh .
RUN ./bullseye-1.sh
RUN apt-get install -qqy wget xz-utils file
COPY etc/bullseye-2.sh .
RUN ./bullseye-2.sh
WORKDIR /rdk
