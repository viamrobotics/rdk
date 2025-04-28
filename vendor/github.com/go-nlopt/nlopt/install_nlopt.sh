#!/bin/sh
set -ex
wget https://codeload.github.com/stevengj/nlopt/tar.gz/v2.7.0
tar -xzvf v2.7.0
cd nlopt-2.7.0 && mkdir build && cd build && cmake -DCMAKE_INSTALL_PREFIX=/usr .. && make && sudo make install && cd ../..
