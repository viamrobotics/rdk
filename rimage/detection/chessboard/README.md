# Chessboard detection

## Usage
The command line tool is at `rimage/cmd/chessboard/main.go`
In order to build and run the command line tool:
```shell
go build rimage/cmd/chessboard/main.go
./chessboard [--path <path/to/image>] [--conf <path/to/config/file>]
```

## Description of algorithm
1. Convert image to gray
2. Blur
3. Compute map of saddle points presence
4. Compute edges and contour
5. For each contour, create most probable chess grid and keep the one with the most inliers

### Pre-processing (1 and 2)
Input image is converted to a Luminance image, and blur with a Gaussian kernel of size 3x3 and `sigma=0.5`


### Saddle points map (3)
1. Compute negative hessian with Sobel operator : `-(gxx * gyy - gyx**2)`
2. Remove points in the lowest quantile
3. Prune points according to the intensity of local maxima
4. Non-maximum suppression of saddle map

### Edges and Contours (4)
1. Compute connectivity 8 Canny edges
2. Perform morphological gradient with 3x3 cross structuring elements to get thicker edges and be able to extract contours
3. Find contours in previous image with the algorithm from Suzuki and Abe
4. Approximate contours with multi-lines with the Douglas-Peucker algorithm
5. Prune contours in order to only keep square contours (4 sides with angle constraints)

### Greedy grid estimation (5)
For each contour:
1. get homography between unit quad and current contour (also a quad)
2. for each 7 possible grid positions, compute current grid and get the number of points that are saddle points for each
3. keep grid position with the most good points

## Possible improvements
- [ ] discriminate saddle points by histogram of colors in a patch around them
- [ ] more automatic parameter computation
- [ ] display function
- [ ] Mean-Shift Belief Propagation for more accurate squares location