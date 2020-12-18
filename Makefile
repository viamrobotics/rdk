
format:
	gofmt -w .
	clang-format -i --style="{BasedOnStyle: Google, IndentWidth: 4}" `find . -iname "*.cpp" -o -iname "*.h" -o -iname "*.ino"`

docker:
	docker build -f Dockerfile.fortest -t 'echolabs/robotcoretest:latest' .
	docker push 'echolabs/robotcoretest:latest'

