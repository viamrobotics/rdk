
format:
	gofmt -w .
	clang-format -i --style="{BasedOnStyle: Google, IndentWidth: 4}" `find . -iname "*.cpp" -o -iname "*.h" -o -iname "*.ino"`
