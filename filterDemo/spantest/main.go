package main

import (
	"context"
	"fmt"

	"go.opencensus.io/trace"
)

func main() {
	fmt.Println("hello world")

	ctx := context.WithValue(context.Background(), "katKey", "katVal")
	fmt.Println("original ctx val", ctx.Value("katKey"))

	childCtx, _ := trace.StartSpan(ctx, "spann...")
	fmt.Println("child ctx val", childCtx.Value("katKey"))
}
