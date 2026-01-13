module go.viam.com/rdk

go 1.25.1

// This must be a replace because a bunch of our deps also use it + `go mod tidy` fails from the conflict
// if you switch it out in the require block.
// We fork this bc the stock version of this library is over 20mb.
replace github.com/hashicorp/go-getter => github.com/viam-labs/go-getter v0.0.0-20251022162721-98d73b852c8a

require (
	github.com/AlekSi/gocov-xml v1.0.0
	github.com/Masterminds/semver/v3 v3.3.1
	github.com/a8m/envsubst v1.4.2
	github.com/axw/gocov v1.1.0
	github.com/aybabtme/uniplot v0.0.0-20151203143629-039c559e5e7e
	github.com/benbjohnson/clock v1.3.5
	github.com/bep/debounce v1.2.1
	github.com/bluenviron/gortsplib/v4 v4.8.0
	github.com/bluenviron/mediacommon v1.9.2
	github.com/bufbuild/buf v1.30.0
	github.com/charmbracelet/huh v0.6.0
	github.com/charmbracelet/huh/spinner v0.0.0-20240917123815-c9b2c9cdb7b6
	github.com/chenzhekl/goply v0.0.0-20190930133256-258c2381defd
	github.com/creack/pty v1.1.20
	github.com/disintegration/imaging v1.6.2
	github.com/docker/go-units v0.5.0
	github.com/edaniels/gobag v1.0.7-0.20220607183102-4242cd9e2848
	github.com/edaniels/golog v0.0.0-20250821172758-0d08e67686a9
	github.com/edaniels/lidario v0.0.0-20220607182921-5879aa7b96dd
	github.com/fatih/color v1.18.0
	github.com/fogleman/gg v1.3.0
	github.com/fsnotify/fsnotify v1.9.0
	github.com/fullstorydev/grpcurl v1.8.6
	github.com/go-co-op/gocron/v2 v2.18.0
	github.com/go-git/go-git/v5 v5.16.2
	github.com/go-gl/mathgl v1.0.0
	github.com/go-nlopt/nlopt v0.0.0-20230219125344-443d3362dcb5
	github.com/go-viper/mapstructure/v2 v2.4.0
	github.com/goccy/go-graphviz v0.1.3
	github.com/golang-jwt/jwt/v4 v4.5.2
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0
	github.com/golang/geo v0.0.0-20230421003525-6adc56603217
	github.com/golang/protobuf v1.5.4
	github.com/google/go-cmp v0.7.0
	github.com/google/uuid v1.6.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.4.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.2
	github.com/hashicorp/go-getter v1.8.3
	github.com/iancoleman/orderedmap v0.3.0
	github.com/invopop/jsonschema v0.6.0
	github.com/jedib0t/go-pretty/v6 v6.4.6
	github.com/jhump/protoreflect v1.15.6
	github.com/kellydunn/golang-geo v0.7.0
	github.com/kylelemons/godebug v1.1.0
	github.com/lestrrat-go/jwx v1.2.29
	github.com/lmittmann/ppm v1.0.2
	github.com/lucasb-eyer/go-colorful v1.2.0
	github.com/matttproud/golang_protobuf_extensions v1.0.4
	github.com/mkch/gpio v0.0.0-20190919032813-8327cd97d95e
	github.com/montanaflynn/stats v0.7.1
	github.com/muesli/clusters v0.0.0-20200529215643-2700303c1762
	github.com/muesli/kmeans v0.3.1
	github.com/nathan-fiscaletti/consolesize-go v0.0.0-20220204101620-317176b6684d
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/pion/interceptor v0.1.42
	github.com/pion/logging v0.2.4
	github.com/pion/mediadevices v0.9.0
	github.com/pion/rtp v1.8.26
	github.com/pion/stun v0.6.1
	github.com/prometheus/procfs v0.15.1
	github.com/pterm/pterm v0.12.82
	github.com/rhysd/actionlint v1.7.8
	github.com/rs/cors v1.11.1
	github.com/samber/lo v1.51.0
	github.com/sergi/go-diff v1.4.0
	github.com/shirou/gopsutil/v3 v3.24.5
	github.com/spf13/cast v1.5.0
	github.com/u2takey/ffmpeg-go v0.4.1
	github.com/urfave/cli/v2 v2.10.3
	github.com/viam-labs/motion-tools v0.19.2
	github.com/viamrobotics/evdev v0.1.3
	github.com/viamrobotics/webrtc/v3 v3.99.16
	github.com/xfmoulet/qoi v0.2.0
	github.com/zhuyie/golzf v0.0.0-20161112031142-8387b0307ade
	go-hep.org/x/hep v0.32.1
	go.mongodb.org/mongo-driver v1.17.1
	go.opencensus.io v0.24.0
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.54.0
	go.opentelemetry.io/otel v1.38.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.38.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.38.0
	go.opentelemetry.io/otel/sdk v1.38.0
	go.opentelemetry.io/proto/otlp v1.9.0
	go.uber.org/atomic v1.11.0
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	go.viam.com/api v0.1.503
	go.viam.com/test v1.2.4
	go.viam.com/utils v0.4.3
	goji.io v2.0.2+incompatible
	golang.org/x/image v0.25.0
	golang.org/x/mobile v0.0.0-20240112133503-c713f31d574b
	golang.org/x/sync v0.18.0
	golang.org/x/sys v0.38.0
	golang.org/x/term v0.37.0
	golang.org/x/text v0.31.0
	golang.org/x/time v0.6.0
	golang.org/x/tools v0.39.0
	gonum.org/v1/gonum v0.16.0
	gonum.org/v1/plot v0.15.2
	google.golang.org/genproto/googleapis/api v0.0.0-20250825161204-c5933d9347a5
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250825161204-c5933d9347a5
	google.golang.org/grpc v1.75.1
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.5.1
	google.golang.org/protobuf v1.36.10
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
	gorgonia.org/tensor v0.9.24
	gotest.tools/gotestsum v1.12.2
	periph.io/x/conn/v3 v3.7.0
	periph.io/x/host/v3 v3.8.1-0.20230331112814-9f0d9f7d76db
)

require (
	atomicgo.dev/cursor v0.2.0 // indirect
	atomicgo.dev/keyboard v0.2.9 // indirect
	atomicgo.dev/schedule v0.1.0 // indirect
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.33.0-20240221180331-f05a6f4403ce.1 // indirect
	cel.dev/expr v0.24.0 // indirect
	cloud.google.com/go v0.115.1 // indirect
	cloud.google.com/go/auth v0.9.3 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.4 // indirect
	cloud.google.com/go/compute/metadata v0.7.0 // indirect
	cloud.google.com/go/container v1.39.0 // indirect
	cloud.google.com/go/iam v1.2.0 // indirect
	cloud.google.com/go/monitoring v1.21.0 // indirect
	cloud.google.com/go/storage v1.43.0 // indirect
	cloud.google.com/go/trace v1.11.0 // indirect
	codeberg.org/go-fonts/liberation v0.5.0 // indirect
	codeberg.org/go-latex/latex v0.1.0 // indirect
	codeberg.org/go-pdf/fpdf v0.10.0 // indirect
	connectrpc.com/connect v1.15.0 // indirect
	connectrpc.com/otelconnect v0.7.0 // indirect
	contrib.go.opencensus.io/exporter/stackdriver v0.13.4 // indirect
	git.sr.ht/~sbinet/gg v0.6.0 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ajstarks/svgo v0.0.0-20211024235047-1546f124cd8b // indirect
	github.com/antlr4-go/antlr/v4 v4.13.0 // indirect
	github.com/apache/arrow/go/arrow v0.0.0-20201229220542-30ce2eb5d4dc // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/aws/aws-sdk-go v1.38.20 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/bgentry/go-netrc v0.0.0-20140422174119-9fd32a8b3d3d // indirect
	github.com/bitfield/gotestdox v0.2.2 // indirect
	github.com/blackjack/webcam v0.6.1 // indirect
	github.com/bmatcuk/doublestar/v4 v4.9.1 // indirect
	github.com/bufbuild/protocompile v0.9.0 // indirect
	github.com/bufbuild/protovalidate-go v0.6.0 // indirect
	github.com/bufbuild/protoyaml-go v0.1.8 // indirect
	github.com/bytedance/sonic v1.13.1 // indirect
	github.com/campoy/embedmd v1.0.0 // indirect
	github.com/catppuccin/go v0.2.0 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/charmbracelet/bubbles v0.20.0 // indirect
	github.com/charmbracelet/bubbletea v1.1.1 // indirect
	github.com/charmbracelet/colorprofile v0.2.3-0.20250311203215-f60798e515dc // indirect
	github.com/charmbracelet/lipgloss v1.1.0 // indirect
	github.com/charmbracelet/x/ansi v0.8.0 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.13-0.20250311204145-2c3ea96c31dd // indirect
	github.com/charmbracelet/x/exp/strings v0.0.0-20240722160745-212f7b056ed0 // indirect
	github.com/charmbracelet/x/term v0.2.1 // indirect
	github.com/chewxy/hm v1.0.0 // indirect
	github.com/chewxy/math32 v1.0.8 // indirect
	github.com/clipperhouse/stringish v0.1.1 // indirect
	github.com/clipperhouse/uax29/v2 v2.3.0 // indirect
	github.com/cncf/xds/go v0.0.0-20250501225837-2ac532fd4443 // indirect
	github.com/containerd/console v1.0.5 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.15.1 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.2.0 // indirect
	github.com/desertbit/timer v0.0.0-20180107155436-c41aec40b27f // indirect
	github.com/dgottlieb/smarty-assertions v1.2.6 // indirect
	github.com/distribution/reference v0.5.0 // indirect
	github.com/dnephin/pflag v1.0.7 // indirect
	github.com/docker/cli v25.0.4+incompatible // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker v25.0.6+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.8.1 // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.32.4 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.2.1 // indirect
	github.com/erikgeiser/coninput v0.0.0-20211004153227-1c3628e74d0f // indirect
	github.com/felixge/fgprof v0.9.4 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/gen2brain/malgo v0.11.24 // indirect
	github.com/gin-gonic/gin v1.9.1 // indirect
	github.com/go-chi/chi/v5 v5.2.2 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.6.2 // indirect
	github.com/go-jose/go-jose/v4 v4.1.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/gofrs/uuid/v5 v5.0.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/gonuts/binary v0.2.0 // indirect
	github.com/google/cel-go v0.20.1 // indirect
	github.com/google/flatbuffers v2.0.6+incompatible // indirect
	github.com/google/go-containerregistry v0.19.0 // indirect
	github.com/google/pprof v0.0.0-20240827171923-fa2c70bbbfe5 // indirect
	github.com/google/s2a-go v0.1.8 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.3 // indirect
	github.com/googleapis/gax-go/v2 v2.13.0 // indirect
	github.com/gookit/color v1.5.4 // indirect
	github.com/gorilla/securecookie v1.1.2 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/improbable-eng/grpc-web v0.15.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jdx/go-netrc v1.0.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/lestrrat-go/backoff/v2 v2.0.8 // indirect
	github.com/lestrrat-go/blackmagic v1.0.2 // indirect
	github.com/lestrrat-go/httpcc v1.0.1 // indirect
	github.com/lestrrat-go/iter v1.0.2 // indirect
	github.com/lestrrat-go/option v1.0.1 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/lithammer/fuzzysearch v1.1.8 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-localereader v0.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.19 // indirect
	github.com/mattn/go-shellwords v1.0.12 // indirect
	github.com/miekg/dns v1.1.53 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/hashstructure/v2 v2.0.2 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/muesli/ansi v0.0.0-20230316100256-276c6243b2f6 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/muhlemmer/gu v0.3.1 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0 // indirect
	github.com/pierrec/lz4 v2.0.5+incompatible // indirect
	github.com/pion/datachannel v1.5.10 // indirect
	github.com/pion/dtls/v2 v2.2.12 // indirect
	github.com/pion/dtls/v3 v3.0.8 // indirect
	github.com/pion/ice/v4 v4.0.13 // indirect
	github.com/pion/mdns v0.0.12 // indirect
	github.com/pion/mdns/v2 v2.1.0 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/rtcp v1.2.16 // indirect
	github.com/pion/sctp v1.8.41 // indirect
	github.com/pion/sdp/v3 v3.0.16 // indirect
	github.com/pion/srtp/v2 v2.0.20 // indirect
	github.com/pion/srtp/v3 v3.0.9 // indirect
	github.com/pion/stun/v3 v3.0.2 // indirect
	github.com/pion/transport/v2 v2.2.10 // indirect
	github.com/pion/transport/v3 v3.1.1 // indirect
	github.com/pion/turn/v2 v2.1.6 // indirect
	github.com/pion/turn/v4 v4.1.3 // indirect
	github.com/pion/webrtc/v4 v4.1.8 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/profile v1.7.0 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spf13/cobra v1.10.1 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/spiffe/go-spiffe/v2 v2.5.0 // indirect
	github.com/srikrsna/protoc-gen-gotag v0.6.2 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/u2takey/go-utils v0.3.1 // indirect
	github.com/ulikunitz/xz v0.5.15 // indirect
	github.com/vbatts/tar-split v0.11.5 // indirect
	github.com/viamrobotics/ice/v2 v2.3.40 // indirect
	github.com/viamrobotics/zeroconf v1.0.13 // indirect
	github.com/wlynxg/anet v0.0.5 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	github.com/xtgo/set v1.0.0 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	github.com/zeebo/errs v1.4.0 // indirect
	github.com/zitadel/oidc/v3 v3.37.0 // indirect
	github.com/zitadel/schema v1.3.1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.54.0 // indirect
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	go.yaml.in/yaml/v4 v4.0.0-rc.2 // indirect
	go4.org/unsafe/assume-no-moving-gc v0.0.0-20230525183740-e7c30c78aeb2 // indirect
	golang.org/x/arch v0.23.0 // indirect
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/mod v0.30.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2 // indirect
	google.golang.org/api v0.196.0 // indirect
	google.golang.org/genproto v0.0.0-20240903143218-8af14fe29dc1 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gorgonia.org/vecf32 v0.9.0 // indirect
	gorgonia.org/vecf64 v0.9.0 // indirect
	nhooyr.io/websocket v1.8.7 // indirect
)

require (
	github.com/erikstmartin/go-testdb v0.0.0-20160219214506-8d10e4a1bae5 // indirect
	github.com/kylelemons/go-gypsy v1.0.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/yosuke-furukawa/json5 v0.1.1
	github.com/ziutek/mymysql v1.5.4 // indirect
	golang.org/x/exp v0.0.0-20251113190631-e25ba8c21ef6
)
