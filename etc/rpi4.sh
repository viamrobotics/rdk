sudo systemctl enable ssh
sudo apt -y update
sudo apt -y upgrade
sudo apt -y install cheese  wiringpi git autoconf swig yasm libpython2.7-dev cmake libxext-dev git v4l-utils apt-file pigpio-tools libx264-dev libmicrohttpd-dev libtool libpigpio-dev
sudo apt-file update

# optinal and only sometimes works
sudo apt -y install streamer

cd /usr/local
sudo wget https://golang.org/dl/go1.16.linux-arm64.tar.gz
sudo tar zxvf go1.16.linux-arm64.tar.gz
sudo rm go1.16.linux-arm64.tar.gz
sudo ln -s /usr/local/go/bin/* /usr/local/bin/

cd ~
mkdir stuff
cd stuff

cd ~/stuff
wget -q --no-check-certificate https://codeload.github.com/stevengj/nlopt/tar.gz/v2.7.0
tar -xzvf v2.7.0 && rm -f v2.7.0
cd nlopt-2.7.0 && mkdir build && cd build && cmake -DCMAKE_INSTALL_PREFIX=/usr .. && make -j 4 && sudo make install

cd ~/stuff
sudo apt-get -y remove wiringpi
git clone https://github.com/WiringPi/WiringPi.git
cd WiringPi
./build

cd ~/stuff
git clone https://github.com/webmproject/libvpx.git
cd libvpx
cd build
../configure --enable-runtime-cpu-detect --enable-vp8 --enable-postproc --enable-multi-res-encoding --enable-webm-io --enable-better-hw-compatibility --enable-onthefly-bitpacking --enable-pic
sudo make -j4 install
