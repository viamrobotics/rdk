module {{ .ModuleLowercase }}

go 1.23

require (
	go.viam.com/rdk v{{ .SDKVersion }}
)
