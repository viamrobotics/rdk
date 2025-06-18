# clusters

Data structs and algorithms for clustering data observations and basic
computations in n-dimensional spaces.

## Example

```go
import "github.com/muesli/clusters"

// fake some observations
var o clusters.Observations
o = append(o, clusters.Coordinates{1, 1})
o = append(o, clusters.Coordinates{3, 2})
o = append(o, clusters.Coordinates{5, 3})

// seed a new set of clusters
c, err := clusters.New(2, o)

// add observations to clusters
c[0].Append(o[0])
c[1].Append(o[1])
c[1].Append(o[2])

// calculate the centroids for each cluster
c.Recenter()

// find the nearest cluster for an observation
i := c.Nearest(o[1])
// => returns index 1

// find the neighbouring cluster and its average distance for an observation
i, d := c.Neighbour(o[0], 0)
// => returns index 1 with euclidean distance 12.5
```

## Development

[![GoDoc](https://godoc.org/github.com/golang/gddo?status.svg)](https://godoc.org/github.com/muesli/clusters)
[![Build Status](https://travis-ci.org/muesli/clusters.svg?branch=master)](https://travis-ci.org/muesli/clusters)
[![Coverage Status](https://coveralls.io/repos/github/muesli/clusters/badge.svg?branch=master)](https://coveralls.io/github/muesli/clusters?branch=master)
[![Go ReportCard](http://goreportcard.com/badge/muesli/clusters)](http://goreportcard.com/report/muesli/clusters)
