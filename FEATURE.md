# aw/cli-size feature branch

This is a feature branch to shrink the size of the CLI.

```sh
$ make cli-ci
$ du -hc bin/linux-amd64/viam-cli
59M     bin/linux-amd64/viam-cli
59M     total
```

```sh
$ make server-static-compressed
$ du -hc bin/Linux-x86_64/viam-server-static*
75M     bin/Linux-x86_64/viam-server-static
19M     bin/Linux-x86_64/viam-server-static-compressed
94M     total
```

The CLI is a smaller codebase than server-static but is nearly as large of a binary before compression. I never know with go binaries but it seems big.

Reasons I can think of:
- not stripping the binary
- a large dependency

Can you investigate these and, if you like, other potential causes of the binary size and try to shrink it.

We don't compress the CLI now because of concerns about startup time with UPX. Can you test this theory?
