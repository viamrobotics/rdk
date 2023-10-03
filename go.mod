module go.viam.com/rdk

go 1.19

require (
	github.com/AlekSi/gocov-xml v1.0.0
	github.com/CPRT/roboclaw v0.0.0-20190825181223-76871438befc
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/NYTimes/gziphandler v1.1.1
	github.com/a8m/envsubst v1.4.2
	github.com/adrianmo/go-nmea v1.7.0
	github.com/aler9/gortsplib/v2 v2.1.0
	github.com/axw/gocov v1.1.0
	github.com/aybabtme/uniplot v0.0.0-20151203143629-039c559e5e7e
	github.com/benbjohnson/clock v1.3.3
	github.com/bep/debounce v1.2.1
	github.com/bufbuild/buf v1.6.0
	github.com/creack/pty v1.1.19-0.20220421211855-0d412c9fbeb1
	github.com/d2r2/go-i2c v0.0.0-20191123181816-73a8a799d6bc
	github.com/d2r2/go-logger v0.0.0-20210606094344-60e9d1233e22
	github.com/de-bkg/gognss v0.0.0-20220601150219-24ccfdcdbb5d
	github.com/disintegration/imaging v1.6.2
	github.com/docker/go-units v0.5.0
	github.com/edaniels/gobag v1.0.7-0.20220607183102-4242cd9e2848
	github.com/edaniels/golinters v0.0.5-0.20220906153528-641155550742
	github.com/edaniels/golog v0.0.0-20230215213219-28954395e8d0
	github.com/edaniels/lidario v0.0.0-20220607182921-5879aa7b96dd
	github.com/erh/scheme v0.0.0-20210304170849-99d295c6ce9a
	github.com/fatih/color v1.15.0
	github.com/fogleman/gg v1.3.0
	github.com/fsnotify/fsnotify v1.6.0
	github.com/fullstorydev/grpcurl v1.8.6
	github.com/go-audio/audio v1.0.0
	github.com/go-audio/transforms v0.0.0-20180121090939-51830ccc35a5
	github.com/go-audio/wav v1.1.0
	github.com/go-gl/mathgl v1.0.0
	github.com/go-gnss/rtcm v0.0.3
	github.com/go-nlopt/nlopt v0.0.0-20230219125344-443d3362dcb5
	github.com/golang-jwt/jwt/v4 v4.5.0
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0
	github.com/golang/geo v0.0.0-20210211234256-740aa86cb551
	github.com/golang/protobuf v1.5.3
	github.com/golangci/golangci-lint v1.51.2
	github.com/google/flatbuffers v2.0.6+incompatible
	github.com/google/go-cmp v0.5.9
	github.com/google/uuid v1.3.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.4.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.15.2
	github.com/invopop/jsonschema v0.6.0
	github.com/jacobsa/go-serial v0.0.0-20180131005756-15cf729a72d4
	github.com/jedib0t/go-pretty/v6 v6.4.6
	github.com/jhump/protoreflect v1.15.1
	github.com/kellydunn/golang-geo v0.7.0
	github.com/lestrrat-go/jwx v1.2.25
	github.com/lmittmann/ppm v1.0.2
	github.com/lucasb-eyer/go-colorful v1.2.0
	github.com/mattn/go-tflite v1.0.4
	github.com/matttproud/golang_protobuf_extensions v1.0.4
	github.com/mitchellh/copystructure v1.2.0
	github.com/mitchellh/mapstructure v1.5.0
	github.com/mkch/gpio v0.0.0-20190919032813-8327cd97d95e
	github.com/montanaflynn/stats v0.7.0
	github.com/muesli/clusters v0.0.0-20200529215643-2700303c1762
	github.com/muesli/kmeans v0.3.1
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/pion/mediadevices v0.5.1-0.20230724160738-03c44ee80347
	github.com/pion/rtp v1.7.13
	github.com/pion/webrtc/v3 v3.2.11
	github.com/rhysd/actionlint v1.6.24
	github.com/rs/cors v1.9.0
	github.com/sergi/go-diff v1.3.1
	github.com/sjwhitworth/golearn v0.0.0-20211014193759-a8b69c276cd8
	github.com/u2takey/ffmpeg-go v0.4.1
	github.com/urfave/cli/v2 v2.10.3
	github.com/viam-labs/go-libjpeg v0.3.1
	github.com/viamrobotics/evdev v0.1.3
	github.com/viamrobotics/gostream v0.0.0-20230725145737-ed58004e202e
	github.com/xfmoulet/qoi v0.2.0
	go-hep.org/x/hep v0.32.1
	go.einride.tech/vlp16 v0.7.0
	go.mongodb.org/mongo-driver v1.11.6
	go.opencensus.io v0.24.0
	go.uber.org/atomic v1.10.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.24.0
	go.viam.com/api v0.1.203
	go.viam.com/test v1.1.1-0.20220913152726-5da9916c08a2
	go.viam.com/utils v0.1.46
	goji.io v2.0.2+incompatible
	golang.org/x/image v0.8.0
	golang.org/x/tools v0.8.0
	gonum.org/v1/gonum v0.12.0
	gonum.org/v1/plot v0.12.0
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1
	google.golang.org/grpc v1.54.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.2.0
	google.golang.org/protobuf v1.30.0
	gopkg.in/src-d/go-billy.v4 v4.3.2
	gorgonia.org/tensor v0.9.24
	gotest.tools/gotestsum v1.10.0
	periph.io/x/conn/v3 v3.7.0
	periph.io/x/host/v3 v3.8.1-0.20230331112814-9f0d9f7d76db
)

require (
	4d63.com/gocheckcompilerdirectives v1.2.1 // indirect
	4d63.com/gochecknoglobals v0.2.1 // indirect
	cloud.google.com/go v0.110.0 // indirect
	cloud.google.com/go/compute v1.19.0 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	cloud.google.com/go/container v1.15.0 // indirect
	cloud.google.com/go/iam v0.13.0 // indirect
	cloud.google.com/go/monitoring v1.13.0 // indirect
	cloud.google.com/go/storage v1.30.1 // indirect
	cloud.google.com/go/trace v1.9.0 // indirect
	contrib.go.opencensus.io/exporter/stackdriver v0.13.4 // indirect
	git.sr.ht/~sbinet/gg v0.3.1 // indirect
	github.com/Abirdcfly/dupword v0.0.9 // indirect
	github.com/Antonboom/errname v0.1.7 // indirect
	github.com/Antonboom/nilnil v0.1.1 // indirect
	github.com/BurntSushi/toml v1.2.1 // indirect
	github.com/Djarvur/go-err113 v0.0.0-20210108212216-aea10b59be24 // indirect
	github.com/GaijinEntertainment/go-exhaustruct/v2 v2.3.0 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/OpenPeeDeeP/depguard v1.1.1 // indirect
	github.com/ajstarks/svgo v0.0.0-20211024235047-1546f124cd8b // indirect
	github.com/alecthomas/participle/v2 v2.0.0-alpha3 // indirect
	github.com/alexkohler/prealloc v1.0.0 // indirect
	github.com/alingse/asasalint v0.0.11 // indirect
	github.com/apache/arrow/go/arrow v0.0.0-20201229220542-30ce2eb5d4dc // indirect
	github.com/ashanbrown/forbidigo v1.4.0 // indirect
	github.com/ashanbrown/makezero v1.1.1 // indirect
	github.com/aws/aws-sdk-go v1.38.20 // indirect
	github.com/bamiaux/iobit v0.0.0-20170418073505-498159a04883 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bkielbasa/cyclop v1.2.0 // indirect
	github.com/blackjack/webcam v0.0.0-20230509180125-87693b3f29dc // indirect
	github.com/blizzy78/varnamelen v0.8.0 // indirect
	github.com/bombsimon/wsl/v3 v3.4.0 // indirect
	github.com/breml/bidichk v0.2.3 // indirect
	github.com/breml/errchkjson v0.3.0 // indirect
	github.com/bufbuild/connect-go v0.1.1 // indirect
	github.com/bufbuild/protocompile v0.5.1 // indirect
	github.com/butuzov/ireturn v0.1.1 // indirect
	github.com/campoy/embedmd v1.0.0 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/charithe/durationcheck v0.0.9 // indirect
	github.com/chavacava/garif v0.0.0-20221024190013-b3ef35877348 // indirect
	github.com/chewxy/hm v1.0.0 // indirect
	github.com/chewxy/math32 v1.0.8 // indirect
	github.com/cncf/udpa/go v0.0.0-20220112060539-c52dc94e7fbe // indirect
	github.com/cncf/xds/go v0.0.0-20230105202645-06c439db220b // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/curioswitch/go-reassign v0.2.0 // indirect
	github.com/daixiang0/gci v0.9.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.2.0 // indirect
	github.com/denis-tingaikin/go-header v0.4.3 // indirect
	github.com/desertbit/timer v0.0.0-20180107155436-c41aec40b27f // indirect
	github.com/dnephin/pflag v1.0.7 // indirect
	github.com/edaniels/zeroconf v1.0.10 // indirect
	github.com/envoyproxy/go-control-plane v0.10.3 // indirect
	github.com/envoyproxy/protoc-gen-validate v0.9.1 // indirect
	github.com/esimonov/ifshort v1.0.4 // indirect
	github.com/ettle/strcase v0.1.1 // indirect
	github.com/fatih/camelcase v1.0.0 // indirect
	github.com/fatih/structtag v1.2.0 // indirect
	github.com/firefart/nonamedreturns v1.0.4 // indirect
	github.com/fzipp/gocyclo v0.6.0 // indirect
	github.com/gen2brain/malgo v0.11.10 // indirect
	github.com/gin-gonic/gin v1.8.1 // indirect
	github.com/go-audio/riff v1.0.0 // indirect
	github.com/go-chi/chi/v5 v5.0.7 // indirect
	github.com/go-critic/go-critic v0.6.7 // indirect
	github.com/go-fonts/liberation v0.3.0 // indirect
	github.com/go-latex/latex v0.0.0-20230307184459-12ec69307ad9 // indirect
	github.com/go-pdf/fpdf v0.6.0 // indirect
	github.com/go-restruct/restruct v1.2.0-alpha.0.20210525045353-983b86fa188e // indirect
	github.com/go-toolsmith/astcast v1.1.0 // indirect
	github.com/go-toolsmith/astcopy v1.0.3 // indirect
	github.com/go-toolsmith/astequal v1.1.0 // indirect
	github.com/go-toolsmith/astfmt v1.1.0 // indirect
	github.com/go-toolsmith/astp v1.1.0 // indirect
	github.com/go-toolsmith/strparse v1.1.0 // indirect
	github.com/go-toolsmith/typep v1.1.0 // indirect
	github.com/go-xmlfmt/xmlfmt v1.1.2 // indirect
	github.com/goburrow/serial v0.1.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/gofrs/flock v0.8.1 // indirect
	github.com/gofrs/uuid v4.2.0+incompatible // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/glog v1.0.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/golangci/check v0.0.0-20180506172741-cfe4005ccda2 // indirect
	github.com/golangci/dupl v0.0.0-20180902072040-3e9179ac440a // indirect
	github.com/golangci/go-misc v0.0.0-20220329215616-d24fe342adfe // indirect
	github.com/golangci/gofmt v0.0.0-20220901101216-f2edd75033f2 // indirect
	github.com/golangci/lint-1 v0.0.0-20191013205115-297bf364a8e0 // indirect
	github.com/golangci/maligned v0.0.0-20180506175553-b1d89398deca // indirect
	github.com/golangci/misspell v0.4.0 // indirect
	github.com/golangci/revgrep v0.0.0-20220804021717-745bb2f7c2e6 // indirect
	github.com/golangci/unconvert v0.0.0-20180507085042-28b1c447d1f4 // indirect
	github.com/gonum/blas v0.0.0-20181208220705-f22b278b28ac // indirect
	github.com/gonum/lapack v0.0.0-20181123203213-e4cdc5a0bff9 // indirect
	github.com/gonum/matrix v0.0.0-20181209220409-c518dec07be9 // indirect
	github.com/gonuts/binary v0.2.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.2.3 // indirect
	github.com/googleapis/gax-go/v2 v2.8.0 // indirect
	github.com/gordonklaus/ineffassign v0.0.0-20230107090616-13ace0543b28 // indirect
	github.com/gorilla/securecookie v1.1.1 // indirect
	github.com/gostaticanalysis/analysisutil v0.7.1 // indirect
	github.com/gostaticanalysis/comment v1.4.2 // indirect
	github.com/gostaticanalysis/forcetypeassert v0.1.0 // indirect
	github.com/gostaticanalysis/nilerr v0.1.1 // indirect
	github.com/guptarohit/asciigraph v0.5.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-version v1.6.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hexops/gotextdiff v1.0.3 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/iancoleman/orderedmap v0.0.0-20190318233801-ac98e3ecb4b0 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/improbable-eng/grpc-web v0.15.0 // indirect
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/jdxcode/netrc v0.0.0-20210204082910-926c7f70242a // indirect
	github.com/jgautheron/goconst v1.5.1 // indirect
	github.com/jhump/protocompile v0.0.0-20220216033700-d705409f108f // indirect
	github.com/jingyugao/rowserrcheck v1.1.1 // indirect
	github.com/jirfag/go-printf-func-name v0.0.0-20200119135958-7558a9eaa5af // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/julz/importas v0.1.0 // indirect
	github.com/junk1tm/musttag v0.4.5 // indirect
	github.com/kisielk/errcheck v1.6.3 // indirect
	github.com/kisielk/gotool v1.0.0 // indirect
	github.com/kkHAIKE/contextcheck v1.1.3 // indirect
	github.com/klauspost/compress v1.16.5 // indirect
	github.com/klauspost/pgzip v1.2.5 // indirect
	github.com/kulti/thelper v0.6.3 // indirect
	github.com/kunwardeep/paralleltest v1.0.6 // indirect
	github.com/kyoh86/exportloopref v0.1.11 // indirect
	github.com/ldez/gomoddirectives v0.2.3 // indirect
	github.com/ldez/tagliatelle v0.4.0 // indirect
	github.com/leonklingele/grouper v1.1.1 // indirect
	github.com/lestrrat-go/backoff/v2 v2.0.8 // indirect
	github.com/lestrrat-go/blackmagic v1.0.1 // indirect
	github.com/lestrrat-go/httpcc v1.0.1 // indirect
	github.com/lestrrat-go/iter v1.0.2 // indirect
	github.com/lestrrat-go/option v1.0.1 // indirect
	github.com/lib/pq v1.10.7 // indirect
	github.com/lufeee/execinquery v1.2.1 // indirect
	github.com/magiconair/properties v1.8.6 // indirect
	github.com/maratori/testableexamples v1.0.0 // indirect
	github.com/maratori/testpackage v1.1.0 // indirect
	github.com/matoous/godox v0.0.0-20210227103229-6504466cf951 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.18 // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.14 // indirect
	github.com/mbilski/exhaustivestruct v1.2.0 // indirect
	github.com/mgechev/revive v1.2.5 // indirect
	github.com/miekg/dns v1.1.53 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moricho/tparallel v0.2.1 // indirect
	github.com/nakabonne/nestif v0.3.1 // indirect
	github.com/nbutton23/zxcvbn-go v0.0.0-20210217022336-fa2cb2858354 // indirect
	github.com/nishanths/exhaustive v0.9.5 // indirect
	github.com/nishanths/predeclared v0.2.2 // indirect
	github.com/nunnatsa/ginkgolinter v0.8.1 // indirect
	github.com/olekukonko/tablewriter v0.0.5 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/pelletier/go-toml/v2 v2.0.5 // indirect
	github.com/pierrec/lz4 v2.0.5+incompatible // indirect
	github.com/pion/datachannel v1.5.5 // indirect
	github.com/pion/dtls/v2 v2.2.7 // indirect
	github.com/pion/ice/v2 v2.3.9 // indirect
	github.com/pion/interceptor v0.1.17 // indirect
	github.com/pion/logging v0.2.2 // indirect
	github.com/pion/mdns v0.0.8-0.20230502060824-17c664ea7d5c // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/rtcp v1.2.10 // indirect
	github.com/pion/sctp v1.8.7 // indirect
	github.com/pion/sdp/v3 v3.0.6 // indirect
	github.com/pion/srtp/v2 v2.0.15 // indirect
	github.com/pion/stun v0.6.1 // indirect
	github.com/pion/transport/v2 v2.2.1 // indirect
	github.com/pion/turn/v2 v2.1.2 // indirect
	github.com/pkg/browser v0.0.0-20210911075715-681adbf594b8 // indirect
	github.com/pkg/profile v1.6.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/polyfloyd/go-errorlint v1.1.0 // indirect
	github.com/prometheus/client_golang v1.12.2 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/quasilyte/go-ruleguard v0.3.19 // indirect
	github.com/quasilyte/gogrep v0.5.0 // indirect
	github.com/quasilyte/regex/syntax v0.0.0-20200407221936-30656e2c4a95 // indirect
	github.com/quasilyte/stdinfo v0.0.0-20220114132959-f7386bf02567 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/robfig/cron v1.2.0 // indirect
	github.com/rocketlaunchr/dataframe-go v0.0.0-20201007021539-67b046771f0b // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/ryancurrah/gomodguard v1.3.0 // indirect
	github.com/ryanrolds/sqlclosecheck v0.4.0 // indirect
	github.com/sanposhiho/wastedassign/v2 v2.0.7 // indirect
	github.com/sashamelentyev/interfacebloat v1.1.0 // indirect
	github.com/sashamelentyev/usestdlibvars v1.23.0 // indirect
	github.com/securego/gosec/v2 v2.15.0 // indirect
	github.com/shazow/go-diff v0.0.0-20160112020656-b6b7b6733b8c // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/sivchari/containedctx v1.0.2 // indirect
	github.com/sivchari/nosnakecase v1.7.0 // indirect
	github.com/sivchari/tenv v1.7.1 // indirect
	github.com/smartystreets/assertions v1.13.0 // indirect
	github.com/sonatard/noctx v0.0.1 // indirect
	github.com/sourcegraph/go-diff v0.7.0 // indirect
	github.com/spf13/afero v1.9.2 // indirect
	github.com/spf13/cast v1.5.0 // indirect
	github.com/spf13/cobra v1.6.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.12.0 // indirect
	github.com/srikrsna/protoc-gen-gotag v0.6.2 // indirect
	github.com/ssgreg/nlreturn/v2 v2.2.1 // indirect
	github.com/stbenjam/no-sprintf-host-port v0.1.1 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	github.com/stretchr/testify v1.8.4 // indirect
	github.com/subosito/gotenv v1.4.1 // indirect
	github.com/t-yuki/gocover-cobertura v0.0.0-20180217150009-aaee18c8195c // indirect
	github.com/tarm/serial v0.0.0-20180830185346-98f6abe2eb07 // indirect
	github.com/tdakkota/asciicheck v0.1.1 // indirect
	github.com/tetafro/godot v1.4.11 // indirect
	github.com/timakin/bodyclose v0.0.0-20221125081123-e39cf3fc478e // indirect
	github.com/timonwong/loggercheck v0.9.3 // indirect
	github.com/tomarrell/wrapcheck/v2 v2.8.0 // indirect
	github.com/tommy-muehle/go-mnd/v2 v2.5.1 // indirect
	github.com/u2takey/go-utils v0.3.1 // indirect
	github.com/ultraware/funlen v0.0.3 // indirect
	github.com/ultraware/whitespace v0.0.5 // indirect
	github.com/uudashr/gocognit v1.0.6 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	github.com/xtgo/set v1.0.0 // indirect
	github.com/yagipy/maintidx v1.0.0 // indirect
	github.com/yeya24/promlinter v0.2.0 // indirect
	github.com/youmark/pkcs8 v0.0.0-20201027041543-1326539a0a0a // indirect
	github.com/zitadel/oidc v1.13.4 // indirect
	gitlab.com/bosi/decorder v0.2.3 // indirect
	go.uber.org/goleak v1.2.1 // indirect
	go4.org/unsafe/assume-no-moving-gc v0.0.0-20230525183740-e7c30c78aeb2 // indirect
	golang.org/x/crypto v0.10.0 // indirect
	golang.org/x/exp/typeparams v0.0.0-20230203172020-98cc5a0785f9 // indirect
	golang.org/x/mod v0.10.0 // indirect
	golang.org/x/net v0.11.0 // indirect
	golang.org/x/oauth2 v0.7.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/sys v0.9.0 // indirect
	golang.org/x/term v0.9.0 // indirect
	golang.org/x/text v0.10.0 // indirect
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2 // indirect
	google.golang.org/api v0.114.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gorgonia.org/vecf32 v0.9.0 // indirect
	gorgonia.org/vecf64 v0.9.0 // indirect
	honnef.co/go/tools v0.4.2 // indirect
	howett.net/plist v1.0.0 // indirect
	mvdan.cc/gofumpt v0.4.0 // indirect
	mvdan.cc/interfacer v0.0.0-20180901003855-c20040233aed // indirect
	mvdan.cc/lint v0.0.0-20170908181259-adc824a0674b // indirect
	mvdan.cc/unparam v0.0.0-20221223090309-7455f1af531d // indirect
	nhooyr.io/websocket v1.8.7 // indirect
)

require (
	github.com/erikstmartin/go-testdb v0.0.0-20160219214506-8d10e4a1bae5 // indirect
	github.com/goburrow/modbus v0.1.0
	github.com/kylelemons/go-gypsy v1.0.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/ziutek/mymysql v1.5.4 // indirect
	golang.org/x/exp v0.0.0-20230321023759-10a507213a29
)
