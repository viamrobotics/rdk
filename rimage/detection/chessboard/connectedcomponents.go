package chessboard

import (
	"gonum.org/v1/gonum/mat"
	"image"
)

func IsPixelWithinBounds(p image.Point, M, N int) bool {
	return p.X >=0 && p.X < N && p.Y >=0 && p.Y < M
}

func GetGridNeighbors(p image.Point, M, N int) []image.Point {
	neighbors := make([]image.Point, 0, 8)
	candidates := []image.Point{
		{1,0},{1,1}, {0,1},{-1,1},
		{-1,0},{-1,-1},{0,-1},{1,-1},
	}
	for _, c := range candidates{
		if IsPixelWithinBounds(c, M,N) {
			neighbors = append(neighbors, c)
		}
	}
	return neighbors
}

func WalkComponentBFS(img *mat.Dense, visited map[image.Point]bool, node image.Point) []image.Point {
	M,N := img.Dims()
	visited[node] = true
	queue := make([]image.Point, 0)
	queue = append(queue, node)
	contour := make([]image.Point, 0)
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		contour = append(contour, p)
		neighbors := GetGridNeighbors(p, M, N)
		for _, nb := range neighbors {
			_, ok := visited[nb]
			if !ok && img.At(nb.Y, nb.X) > 0 {
				visited[nb] = true
				queue = append(queue, nb)
			}
		}
	}
	return contour
}

func GetConnectedComponents(img *mat.Dense) [][]image.Point {
	M,N := img.Dims()
	// keep track of visited pixels
	visited := make(map[image.Point]bool)
	components := make([][]image.Point, 0, M*N)
	for i:=0;i<M;i++{
		for j:=0;j<N;j++{
			node := image.Point{j,i}
			_, ok := visited[node]
			// if pixel has a 1 value and is not visited, new component needs to be visited
			if img.At(i,j) >0 && !ok {
				contour := WalkComponentBFS(img,visited,node)
				components = append(components, contour)
			}
		}
	}
	return components
}