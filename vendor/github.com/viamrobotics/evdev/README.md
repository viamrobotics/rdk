# About evdev

`evdev` is a [`libevdev`][1] inspired Go package for working with Linux input
devices. `evdev` works by making `ioctl` system calls for the Linux
[`input`][2] and [`uinput`][3] subsystems. Because `evdev` is written in pure
Go, it can be used without CGO.

`evdev` is used to poll events from, and send events to `/dev/input/event*`
devices. Additionally, `evdev` provides the ability to create virtual `uinput`
devices that can be used similarly.

`evdev` is a rewrite of the [`github.com/jteeuwen/evdev`][4] package. Most of
the credit for this package goes to `jteeuwen` for the original (and amazing!)
work done.

## Installing

`evdev` can be installed in the usual Go fashion:

```sh
$ go get -u github.com/kenshaw/evdev
```

## Using

Please see the [`examples`][5] directory for more examples.

## Permission Issues

Reading events from input devices or creating virtual `uinput` devices requires
`$USER` to have the appropriate system-level permissions. This can be accomplished
by adding `$USER` to a group with read/write access to `/dev/input/event*` and
`uinput` block devices.

Please refer to your relevant Linux distribution's documentation on adding
`$USER` to the appropriate system group, or otherwise allowing read/write
access to `/dev/input/event*` and `uinput` devices.

**Note:** if adding a group to the current `$USER`, it will be necessary to log
out and log back in before the system recognizes the group membership.

### Ubuntu/Debian

On Ubuntu/Debian systems, the current `$USER` can be added to the `input`
group:

```sh
$ sudo adduser $USER input
```

[1]: https://www.freedesktop.org/wiki/Software/libevdev/
[2]: https://www.kernel.org/doc/html/latest/input/index.html
[3]: https://www.kernel.org/doc/html/latest/input/uinput.html
[4]: https://github.com/jteeuwen/evdev
[5]: /examples
