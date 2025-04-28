# gpio

Golang implementation of linux GPIO interface.

[![GoDoc](https://godoc.org/github.com/mkch/gpio?status.svg)](https://godoc.org/github.com/mkch/gpio)

## Features

- Pure go. No C code. Easy to cross compile(Write code on PC or Mac and deploy the binary on Raspberry Pi etc.).

- Go style API. Receiving GPIO edge events through go channels. Write go code, **NOT** *write c doe with go syntax*.
  
- Full implementation of linux GPIO character device interface. Chip info, line info, reading/setting values, active low, open drain, open source, edge events...

- Tested (on my really old **Raspberry Pi Model B Rev 2**).

- Linux GPIO tools(tools/gpio), aka. *lsgpio*, *gpio-event-mon* and *gpio-hammer*, implemented in go. Serve both as code examples and diagnostic tools. See **samples** directory.

- Legacy GPIO sysfs interface(aka. /sys/class/gpio) supporting. See **gpiosysfs** package.

## Requirements

- Go1.13+
  
- Linux 4.8+ with GPIO interface to deploy and test.

## Troubleshooting

- `build .: cannot find module for path .` , `undefined: gpio.OpenChip` or something like that:

    If you get this error message from `go build` when building the samples or your own project, you probably need to use `GOOS=linux GOARCH=arm GOARM=6 go build` or something like that.

- `bash: ./lsgpio: cannot execute binary file`:
  
  If your shell complains, please run the binary(lsgpio in this example) on linux.

- `failed to set direction of GPIO pin #N: open /sys/class/gpio/gpioN/direction: permission denied`:

  The legacy GPIO sysfs interface may need **root** privilege to operate. Try `sudo`.
