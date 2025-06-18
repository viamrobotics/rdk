#!/usr/bin/env python
import os
import sys
import subprocess
import time
from subprocess import call

try:
    # GOPATH = "/Users/johnlindsay/Documents/programming/GoCode/" # change this as needed
    GOPATH = "/Users/johnlindsay/go/"
    GOOS = 'darwin' # darwin, windows, or linux
    GOARCH = 'amd64' # 386, amd64, or arm
    cleanCode = False
    mode = 'install' # install, build, or test; cross-compilation requires build

    # Change the current directory
    dir_path = os.path.dirname(os.path.realpath(__file__))
    os.chdir(dir_path)

    # Find the GOHOSTOS and GOHOSTARCH
    GOHOSTOS = ""
    GOHOSTARCH = ""
    cmd = "go env"
    ps = subprocess.Popen(cmd, shell=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, bufsize=1, universal_newlines=True)
    while True:
        line = ps.stdout.readline()
        if line != '':
            if 'GOHOSTOS' in line:
                GOHOSTOS = line.split("=")[1].replace("\"", "").strip()
            elif 'GOHOSTARCH' in line:
                GOHOSTARCH = line.split("=")[1].replace("\"", "").strip()
        else:
            break

    # set the GOPATH, GOOS, and GOARCH environment variables
    os.environ['GOPATH'] = GOPATH

    if GOOS != GOHOSTOS:
        os.environ['GOOS'] = GOOS
        mode = 'build' # cross-compilation requires build
    if GOARCH != GOHOSTARCH:
        os.environ['GOARCH'] = GOARCH
        mode = 'build' # cross-compilation requires build

    if cleanCode:
        retcode = call(['go', 'clean'], shell=False)
        if retcode < 0:
            print >>sys.stderr, "Child was terminated by signal", -retcode

    if mode == "build":
        retcode = call(['go', 'build', '-v'], shell=False)
        if retcode < 0:
            print >>sys.stderr, "Child was terminated by signal", -retcode
    elif mode == "install":
        retcode = call(['go', 'install'], shell=False)
        if retcode < 0:
            print >>sys.stderr, "Child was terminated by signal", -retcode
    elif mode == "test":
        retcode = call(['go', 'test', '-v'], shell=False)
        if retcode < 0:
            print >>sys.stderr, "Child was terminated by signal", -retcode

except OSError as e:
    print >>sys.stderr, "Execution failed:", e
