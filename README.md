smartcrop.go
============

smartcrop implementation in Go.

This is a fork of [github.com/muesli/smartcrop](https://github.com/muesli/smartcrop) with dependency on opencv removed.

smartcrop finds good crops for arbitrary images and crop sizes, based on Jonas Wagner's [smartcrop.js](https://github.com/jwagner/smartcrop.js)

![Example](./gopher_example.jpg)
Image: [https://www.flickr.com/photos/usfwspacific/8182486789](https://www.flickr.com/photos/usfwspacific/8182486789) CC BY U.S. Fish & Wildlife

## Installation

    go get github.com/artyom/smartcrop

## Example
```go
package main

import (
	"github.com/artyom/smartcrop"
	"fmt"
	"image"
	_ "image/png"
	"os"
)

func main() {
  fi,err := os.Open("test.png")
  if err != nil {
    log.Fatalf(err.Error())
  }

  defer fi.Close()

  img, _, err := image.Decode(fi)
  if err != nil {
    log.Fatalf(err.Error())
  }

  analyzer := smartcrop.NewAnalyzer()
	topCrop, _ := analyzer.FindBestCrop(img, 250, 250)
	fmt.Printf("Top crop: %+v\n", topCrop)
}
```

Also see the test-cases in crop_test.go for further working examples.