# gocov XML

A tool to generate Go coverage in XML report for using with tools/plugins like Jenkins/Cobertura.

> Table of Contents

- [gocov XML](#gocov-xml)
  - [Installation](#installation)
  - [Usage](#usage)
    - [Examples](#examples)
      - [Generate coverage by passing `gocov` output as input to `gocov-xml`](#generate-coverage-by-passing-gocov-output-as-input-to-gocov-xml)
      - [Specifying optional source](#specifying-optional-source)
  - [Authors](#authors)

This is a simple helper tool for generating XML output in [Cobertura](http://cobertura.sourceforge.net/) format
for CIs like [Jenkins](https://wiki.jenkins-ci.org/display/JENKINS/Cobertura+Plugin), [vsts](https://www.visualstudio.com/team-services) and others
from [github.com/axw/gocov](https://github.com/axw/gocov) output.
The generated XML output is in the latest [coverage-04.dtd](http://cobertura.sourceforge.net/xml/coverage-04.dtd) schema

## Installation

Just type the following to install the program and its dependencies:

For Go 1.17 and above:

```bash
go install github.com/axw/gocov/gocov@latest
go install github.com/AlekSi/gocov-xml@latest
```

For previous Go versions:

```bash
go get github.com/axw/gocov/...
go get github.com/AlekSi/gocov-xml
```

## Usage

> **NOTE**: `gocov-xml` reads data from the standard input.

```bash
gocov [-source <absolute path to source>]
```

Where,

- **`source`**: Absolute path to source. Defaults to the current working directory.

### Examples

#### Generate coverage by passing `gocov` output as input to `gocov-xml`

```bash
gocov test github.com/gorilla/mux | gocov-xml > coverage.xml
```

#### Specifying optional source

```bash
gocov test github.com/gorilla/mux | gocov-xml -source /abs/path/to/source > coverage.xml
```

## Authors

- [Alexey Palazhchenko (AlekSi)](https://github.com/AlekSi)
- [Yukinari Toyota (t-yuki)](https://github.com/t-yuki)
- [Marin Bek (marinbek)](https://github.com/marinbek)
- [Alex Castle (acastle)](https://github.com/acastle)
- [Billy Yao (yaoyaozong)](https://github.com/yaoyaozong)
- [Abhijith DA (abhijithda)](https://github.com/abhijithda)
