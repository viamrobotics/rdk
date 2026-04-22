module {{ .ModuleLowercase }}

go 1.23

require (
	github.com/erh/vmodutils v0.3.11-rc3
	go.viam.com/rdk v{{ .SDKVersion }}
)
