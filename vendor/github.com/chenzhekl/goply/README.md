# goply

A simple ply file loader for Go

https://godoc.org/github.com/chenzhekl/goply

```go
package main

import (
    "strings"

    "github.com/chenzhekl/goply"
)

func main() {
    plyData := `ply
format ascii 1.0           
comment made by Greg Turk  
comment this file is a cube
element vertex 3      
property float x           
property float y           
property float z           
element face 1    
property list uchar int vertex_index
end_header
0 0 0
0 1 0
1 0 0
3 0 1 2`

    reader := strings.NewReader(plyData)
    ply := goply.New(reader)

    vertices := ply.Elements("vertex")
    p := vertices[2].Property("x").(float32)
    
    if len(vertices) != 3 {
        panic("")
    }
    if p != 1.0 {
        panic("")
    }
}
```

# LICENSE

MIT License
