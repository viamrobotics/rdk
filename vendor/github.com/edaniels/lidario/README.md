lidario
=======

Description
-----------

lidario is a simple library for reading and writing LiDAR files stored in LAS format. The library is written using the Go programing language. Use the *build.py* file to build/install the source code. The script can also be used to run the tests.

Example Usage
--------------

```Go
import "github.com/edaniels/lidario"

func main() {
    // Reading a LAS file
    fileName := "testdata/sample.las"
    var lf *lidario.LasFile
    var err error
    lf, err = lidario.NewLasFile(fileName, "r")
    if err != nil {
        fmt.Println(err)
    }
    defer lf.Close()

    // Print the data contained in the LAS Header
    fmt.Printf("%v\n", lf.Header)

    // Print the VLR data
    fmt.Println("VLRs:")
    for _, vlr := range lf.VlrData {
        fmt.Println(vlr)
    }

    // Get the X,Y,Z data for a single point
    x, y, z, err := lf.GetXYZ(1000)
    fmt.Printf("Point %v: (%f, %f, %f) Error: %v\n", j, x, y, z, err)
	
    // Get an entire point, including all parts
    var p lidario.LasPointer
    p, err = lf.LasPoint(1000)
    if err != nil {
        fmt.Println(err)
        t.Fatal()
    }
    fmt.Println("Point format:", p.Format())

    // Read all the points
    oldProgress := -1
    progress := 0
    for i := 0; i < int(lf.Header.NumberPoints); i++ {
        if p, err := lf.LasPoint(i); err != nil {
            fmt.Println(err)
            t.Fatal()
        } else {
            if i < 10 {
                pd := p.PointData()
                fmt.Printf("Point %v: (%f, %f, %f, %v, %v, %f)\n", i, pd.X, pd.Y, pd.X, pd.Intensity, pd.ClassBitField.ClassificationString(), p.GpsTimeData())
            }
            progress = int(100.0 * float64(i) / float64(lf.Header.NumberPoints))
            if progress != oldProgress {
                oldProgress = progress
                if progress%10 == 0 {
                    fmt.Printf("Progress: %v\n", progress)
                }
            }
        }
    }

    // Create a new LAS file
    newFileName := "testdata/newFile.las"
    newLf, err := lidario.InitializeUsingFile(newFileName, lf)
    if err != nil {
        fmt.Println(err)
        t.Fail()
    }
    defer newLf.Close()

    progress = 0
    oldProgress = -1
    for i := 0; i < int(lf.Header.NumberPoints); i++ {
        if p, err := lf.LasPoint(i); err != nil {
            fmt.Println(err)
            t.Fatal()
        } else {
            if p.PointData().Z < 100.0 { // only output the point if the elevation is less than 100.0 m
                newLf.AddLasPoint(p)
            }
        }
        progress = int(100.0 * float64(i) / float64(lf.Header.NumberPoints))
        if progress != oldProgress {
            oldProgress = progress
            if progress%10 == 0 {
                fmt.Printf("Progress: %v\n", progress)
            }
        }
    }
}
```