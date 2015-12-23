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
	"image"
	"image/png"
	"log"
	"os"

	"github.com/artyom/smartcrop"
)

func main() {
	fi, err := os.Open("test.png")
	if err != nil {
		log.Fatalf(err.Error())
	}

	defer fi.Close()

	img, _, err := image.Decode(fi)
	if err != nil {
		log.Fatalf(err.Error())
	}
	rect, err := smartcrop.SmartCrop(img, 250, 250)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("cropping from %v to %v", img.Bounds(), rect)

	if si, ok := img.(subimager); ok {
		of, err := os.Create("output.png")
		if err != nil {
			log.Fatal(err)
		}
		defer of.Close()
		if err := png.Encode(of, si.SubImage(rect)); err != nil {
			log.Fatal(err)
		}
	}
}

type subimager interface {
	SubImage(image.Rectangle) image.Image
}
```

Also see the test cases in crop_test.go for further working examples.
