## Linux setup

### libvpx

git clone https://github.com/webmproject/libvpx
cd libvpx
cd build
../configure --enable-runtime-cpu-detect --enable-vp9 --enable-vp8    --enable-postproc --enable-vp9-postproc --enable-multi-res-encoding --enable-webm-io --enable-better-hw-compatibility --enable-vp9-highbitdepth --enable-onthefly-bitpacking
make -j8
sudo make install
