module go.viam.com/core

go 1.16

require (
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/RobinUS2/golang-moving-average v1.0.0
	github.com/adrianmo/go-nmea v1.3.0
	github.com/aybabtme/uniplot v0.0.0-20151203143629-039c559e5e7e
	github.com/disintegration/imaging v1.6.2
	github.com/edaniels/golinters v0.0.5-0.20210512224240-495d3b8eed19
	github.com/edaniels/golog v0.0.0-20210326173913-16d408aa7a5e
	github.com/edaniels/gostream v0.0.0-20211022021553-dcb5ba36518a
	github.com/erh/egoutil v0.0.10
	github.com/erh/scheme v0.0.0-20210304170849-99d295c6ce9a
	github.com/erikstmartin/go-testdb v0.0.0-20160219214506-8d10e4a1bae5 // indirect
	github.com/fogleman/gg v1.3.0
	github.com/fsnotify/fsnotify v1.4.9
	github.com/fullstorydev/grpcurl v1.8.0
	github.com/gin-gonic/gin v1.7.0 // indirect
	github.com/go-errors/errors v1.4.1
	github.com/go-gl/mathgl v1.0.0
	github.com/go-nlopt/nlopt v0.0.0-20210501073024-ea36b13dd737
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0
	github.com/golang/geo v0.0.0-20210211234256-740aa86cb551
	github.com/golangci/golangci-lint v1.39.0
	github.com/gonum/floats v0.0.0-20181209220543-c233463c7e82
	github.com/gonum/integrate v0.0.0-20181209220457-a422b5c0fdf2 // indirect
	github.com/gonum/internal v0.0.0-20181124074243-f884aa714029 // indirect
	github.com/gonum/stat v0.0.0-20181125101827-41a0da705a5b
	github.com/google/uuid v1.3.0
	github.com/gopherjs/gopherjs v0.0.0-20200217142428-fce0ec30dd00 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.5.0
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/jacobsa/go-serial v0.0.0-20180131005756-15cf729a72d4
	github.com/jblindsay/lidario v0.0.0-20170420150243-bb03e55f9757
	github.com/jhump/protoreflect v1.8.1 // indirect
	github.com/kellydunn/golang-geo v0.7.0
	github.com/klauspost/compress v1.11.7 // indirect
	github.com/kylelemons/go-gypsy v1.0.0 // indirect
	github.com/lmittmann/ppm v1.0.0
	github.com/lucasb-eyer/go-colorful v1.2.0
	github.com/mitchellh/mapstructure v1.4.1
	github.com/muesli/clusters v0.0.0-20200529215643-2700303c1762
	github.com/muesli/kmeans v0.2.1
	github.com/pion/datachannel v1.4.22-0.20210420230629-6daf0fdcfcc0 // indirect
	github.com/pion/mediadevices v0.2.0
	github.com/pion/webrtc/v3 v3.1.5
	github.com/polyfloyd/go-errorlint v0.0.0-20201127212506-19bd8db6546f
	github.com/pseudomuto/protoc-gen-doc v1.3.2
	github.com/sergi/go-diff v1.2.0
	github.com/sjwhitworth/golearn v0.0.0-20201127221938-294d65fca392
	github.com/starship-technologies/gobag v1.0.6
	github.com/tidwall/gjson v1.9.3 // indirect
	github.com/tonyOreglia/glee v0.0.0-20201027095806-ae3f0739ad37
	github.com/u2takey/ffmpeg-go v0.3.0
	github.com/viamrobotics/evdev v0.1.3
	github.com/wasmerio/wasmer-go v1.0.4
	github.com/ziutek/mymysql v1.5.4 // indirect
	go-hep.org/x/hep v0.28.5
	go.einride.tech/vlp16 v0.7.0
	go.mongodb.org/mongo-driver v1.5.3
	go.opencensus.io v0.23.0
	go.uber.org/multierr v1.7.0
	go.uber.org/zap v1.19.1
	go.viam.com/dynamixel v0.0.0-20210415184230-4a447af034c4
	go.viam.com/test v1.1.0
	go.viam.com/utils v0.0.2-0.20211022043729-bfbcdc7c186c
	goji.io v2.0.2+incompatible
	golang.org/x/exp v0.0.0-20201203231725-fa01524bc59d // indirect
	golang.org/x/image v0.0.0-20210628002857-a66eb6448b8d
	golang.org/x/tools v0.1.7
	gonum.org/v1/gonum v0.8.2
	gonum.org/v1/netlib v0.0.0-20201012070519-2390d26c3658 // indirect
	gonum.org/v1/plot v0.8.1
	google.golang.org/genproto v0.0.0-20210617175327-b9e0b3197ced
	google.golang.org/grpc v1.38.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.1.0
	google.golang.org/protobuf v1.26.0
	howett.net/plist v0.0.0-20201203080718-1454fab16a06

)

replace github.com/jblindsay/lidario => github.com/edaniels/lidario v0.0.0-20210216165043-81520ca6a2de

replace github.com/starship-technologies/gobag => github.com/kkufieta/gobag v0.0.0-20210528190924-d8b19286f98e

replace github.com/wasmerio/wasmer-go => github.com/meshplus/wasmer-go v0.0.0-20210817103436-19ec68f8bfe2

replace github.com/pion/mediadevices => github.com/edaniels/mediadevices v0.0.0-20211022001911-e8e6d6110b1b
