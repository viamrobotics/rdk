all: fvb

prepare:
	go get -u github.com/axw/gocov/...
	go get -u github.com/gorilla/mux/...
	gocov test -v github.com/gorilla/mux > mux.json

fvb:
	gofmt -e -s -w .
	go vet .
	go run ./gocov-xml.go < mux.json > coverage.xml
	xmllint --valid --noout coverage.xml
