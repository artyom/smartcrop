package smartcrop

import (
	"fmt"
	"image"
	"log"
	"os"
)

func ExampleCrop() {
	fi, err := os.Open("./samples/gopher.jpg")
	if err != nil {
		log.Fatal(err)
	}
	defer fi.Close()

	img, _, err := image.Decode(fi)
	if err != nil {
		log.Fatal(err)
	}

	topCrop, err := SmartCrop(img, 250, 250)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("best crop is", topCrop)

	type subImager interface {
		SubImage(image.Rectangle) image.Image
	}
	if si, ok := img.(subImager); ok {
		cr := si.SubImage(topCrop)
		fmt.Printf("cropped image dimensions are %d x %d", cr.Bounds().Dx(), cr.Bounds().Dy())
	}
	// Output:
	// best crop is (59,0)-(486,427)
	// cropped image dimensions are 427 x 427
}
