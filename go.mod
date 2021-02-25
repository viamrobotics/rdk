module github.com/viamrobotics/robotcore

go 1.16

require (
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/RobinUS2/golang-moving-average v1.0.0
	github.com/blackjack/webcam v0.0.0-20200313125108-10ed912a8539
	github.com/disintegration/imaging v1.6.2
	github.com/edaniels/golinters v0.0.4
	github.com/edaniels/golog v0.0.0-20210104162753-3254576d0129
	github.com/edaniels/gostream v0.0.0-20210225023148-0bae1cfe8d0d
	github.com/edaniels/test v0.0.0-20210217200115-75fc4288dde0
	github.com/fogleman/gg v1.2.1-0.20190220221249-0403632d5b90
	github.com/go-gl/mathgl v1.0.0
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0
	github.com/golangci/golangci-lint v1.37.1
	github.com/gonum/stat v0.0.0-20181125101827-41a0da705a5b
	github.com/jacobsa/go-serial v0.0.0-20180131005756-15cf729a72d4
	github.com/jblindsay/lidario v0.0.0-20170420150243-bb03e55f9757
	github.com/lmittmann/ppm v1.0.0
	github.com/lucasb-eyer/go-colorful v1.2.0
	github.com/muesli/clusters v0.0.0-20200529215643-2700303c1762
	github.com/muesli/kmeans v0.2.1
	github.com/sbinet/go-python v0.1.0
	github.com/sjwhitworth/golearn v0.0.0-20201127221938-294d65fca392
	github.com/stretchr/testify v1.7.0
	github.com/tonyOreglia/glee v0.0.0-20201027095806-ae3f0739ad37
	github.com/viamrobotics/dynamixel v0.0.0-20210218153524-e52f765b9997
	github.com/viamrobotics/mti v0.0.0-20210225175246-7a083665e7fe
	github.com/viamrobotics/rplidar v0.0.0-20210225174233-a6da398e02b4
	go.mongodb.org/mongo-driver v1.4.4
	gobot.io/x/gobot v1.15.0
	golang.org/x/image v0.0.0-20201208152932-35266b937fa6
	gonum.org/v1/gonum v0.8.2
	howett.net/plist v0.0.0-20201203080718-1454fab16a06
)

replace github.com/jblindsay/lidario => github.com/edaniels/lidario v0.0.0-20210216165043-81520ca6a2de

replace gobot.io/x/gobot => github.com/erh/gobot v0.0.0-20210225151211-f55d7247ce47
