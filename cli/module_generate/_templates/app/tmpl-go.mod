module {{ .ModuleLowercase }}

go 1.22

require (
	github.com/erh/vmodutils v0.3.11-rc1
	go.viam.com/rdk v{{ .SDKVersion }}
)
