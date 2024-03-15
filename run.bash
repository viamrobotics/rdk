set -e
source ~/.viamdevrc

# pkill -9 'modmain|viam-server'

cd robotModules/cameraWebRTC
go build modmain.go

cd ../../
make server-static
./viam-server -config /Users/nicksanford/rdk/robotModules/cameraWebRTC/config.json
