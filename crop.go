/*
 * Copyright (c) 2014 Christian Muehlhaeuser
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 *
 *	Authors:
 *		Christian Muehlhaeuser <muesli@gmail.com>
 *		Michael Wendland <michael@michiwend.com>
 */

/*
Package smartcrop is a pure-Go implementation of content aware image cropping
based on Jonas Wagner's smartcrop.js https://github.com/jwagner/smartcrop.js
*/
package smartcrop

import (
	"errors"
	"image"
	"image/color"
	"io/ioutil"
	"log"
	"math"
	"time"

	"github.com/bamiaux/rez"

	"golang.org/x/image/draw"
)

var skinColor = [3]float64{0.78, 0.57, 0.44}

const (
	detailWeight            = 0.2
	skinBias                = 0.9
	skinBrightnessMin       = 0.2
	skinBrightnessMax       = 1.0
	skinThreshold           = 0.8
	skinWeight              = 1.8
	saturationBrightnessMin = 0.05
	saturationBrightnessMax = 0.9
	saturationThreshold     = 0.4
	saturationBias          = 0.2
	saturationWeight        = 0.3
	scoreDownSample         = 8
	// step * minscale rounded down to the next power of two should be good
	step              = 8
	scaleStep         = 0.1
	minScale          = 0.9
	maxScale          = 1.0
	edgeRadius        = 0.4
	edgeWeight        = -20.0
	outsideImportance = -0.5
	ruleOfThirds      = true
	prescale          = true
	prescaleMin       = 400.00
)

// Score contains values that classify matches
type Score struct {
	Detail     float64
	Saturation float64
	Skin       float64
	Total      float64
}

// Crop contains results
type cropInfo struct {
	X      int
	Y      int
	Width  int
	Height int
	Score  Score
}

//CropSettings contains options to
//change cropping behaviour
type CropSettings struct {
	DebugMode bool
	Log       *log.Logger
}

//Analyzer interface analyzes its struct
//and returns the best possible crop with the given
//width and height
//returns an error if invalid
type Analyzer interface {
	FindBestCrop(img image.Image, width, height int) (image.Rectangle, error)
}

type standardAnalyzer struct {
	cropSettings CropSettings
}

//NewAnalyzer returns a new analyzer with default settings
func NewAnalyzer() Analyzer {
	cropSettings := CropSettings{
		DebugMode: false,
		Log:       log.New(ioutil.Discard, "", 0),
		//Log: log.New(os.Stderr, "smartcrop: ", log.Lshortfile),
	}

	return &standardAnalyzer{cropSettings: cropSettings}
}

//NewAnalyzerWithCropSettings returns a new analyzer with the given settings
func NewAnalyzerWithCropSettings(cropSettings CropSettings) Analyzer {
	if cropSettings.Log == nil {
		cropSettings.Log = log.New(ioutil.Discard, "", 0)
	}
	return &standardAnalyzer{cropSettings: cropSettings}
}

func (o standardAnalyzer) FindBestCrop(img image.Image, width, height int) (image.Rectangle, error) {
	log := o.cropSettings.Log
	if width == 0 && height == 0 {
		return image.Rectangle{}, errors.New("Expect either a height or width")
	}

	scale := math.Min(float64(img.Bounds().Size().X)/float64(width), float64(img.Bounds().Size().Y)/float64(height))

	// resize image for faster processing
	var lowimg *image.RGBA
	var prescalefactor = 1.0

	if prescale {

		//if f := 1.0 / scale / minScale; f < 1.0 {
		//	prescalefactor = f
		//}
		if f := prescaleMin / math.Min(float64(img.Bounds().Size().X), float64(img.Bounds().Size().Y)); f < 1.0 {
			prescalefactor = f
		}
		log.Println(prescalefactor)

		rect := image.Rect(0, 0, int(float64(img.Bounds().Dx())*prescalefactor),
			int(float64(img.Bounds().Dy())*prescalefactor))
		lowimg = image.NewRGBA(rect)
		switch img.(type) {
		case *image.YCbCr, *image.RGBA, *image.NRGBA, *image.Gray:
			if err := rez.Convert(lowimg, img, rez.NewBilinearFilter()); err != nil {
				draw.ApproxBiLinear.Scale(lowimg, lowimg.Bounds(), img, img.Bounds(), draw.Src, nil)
			}
		default:
			draw.ApproxBiLinear.Scale(lowimg, lowimg.Bounds(), img, img.Bounds(), draw.Src, nil)
		}
	} else {
		lowimg = toRGBA(img)
	}

	if o.cropSettings.DebugMode {
		writeImageToPng(lowimg, "./smartcrop_prescale.png")
	}

	cropWidth, cropHeight := chop(float64(width)*scale*prescalefactor), chop(float64(height)*scale*prescalefactor)
	realMinScale := math.Min(maxScale, math.Max(1.0/scale, minScale))

	log.Printf("original resolution: %dx%d\n", img.Bounds().Size().X, img.Bounds().Size().Y)
	log.Printf("scale: %f, cropw: %f, croph: %f, minscale: %f\n", scale, cropWidth, cropHeight, realMinScale)

	topCrop, err := analyse(o.cropSettings, lowimg, cropWidth, cropHeight, realMinScale)
	if err != nil {
		return topCrop, err
	}

	if prescale == true {
		topCrop.Min.X = int(chop(float64(topCrop.Min.X) / prescalefactor))
		topCrop.Min.Y = int(chop(float64(topCrop.Min.Y) / prescalefactor))
		topCrop.Max.X = int(chop(float64(topCrop.Max.X) / prescalefactor))
		topCrop.Max.Y = int(chop(float64(topCrop.Max.Y) / prescalefactor))
	}

	return topCrop.Canon(), nil
}

// SmartCrop applies the smartcrop algorithms on the the given image and returns
// the top crop or an error if something went wrong.
func SmartCrop(img image.Image, width, height int) (image.Rectangle, error) {
	analyzer := NewAnalyzer()
	return analyzer.FindBestCrop(img, width, height)
}

func chop(x float64) float64 {
	if x < 0 {
		return math.Ceil(x)
	}
	return math.Floor(x)
}

func thirds(x float64) float64 {
	x = (math.Mod(x-(1.0/3.0)+1.0, 2.0)*0.5 - 0.5) * 16.0
	return math.Max(1.0-x*x, 0.0)
}

func bounds(l float64) float64 {
	return math.Min(math.Max(l, 0.0), 255)
}

func importance(crop cropInfo, x, y int) float64 {
	if crop.X > x || x >= crop.X+crop.Width || crop.Y > y || y >= crop.Y+crop.Height {
		return outsideImportance
	}

	xf := float64(x-crop.X) / float64(crop.Width)
	yf := float64(y-crop.Y) / float64(crop.Height)

	px := math.Abs(0.5-xf) * 2.0
	py := math.Abs(0.5-yf) * 2.0

	dx := math.Max(px-1.0+edgeRadius, 0.0)
	dy := math.Max(py-1.0+edgeRadius, 0.0)
	d := (dx*dx + dy*dy) * edgeWeight

	s := 1.41 - math.Sqrt(px*px+py*py)
	if ruleOfThirds {
		s += (math.Max(0.0, s+d+0.5) * 1.2) * (thirds(px) + thirds(py))
	}

	return s + d
}

func score(output *image.RGBA, crop cropInfo) Score {
	height := output.Bounds().Dx()
	width := output.Bounds().Dy()
	score := Score{}

	// same loops but with downsampling
	for y := 0; y <= height-scoreDownSample; y += scoreDownSample {
		for x := 0; x <= width-scoreDownSample; x += scoreDownSample {
			c := output.RGBAAt(x, y)
			r8 := float64(c.R)
			g8 := float64(c.G)
			b8 := float64(c.B)
			imp := importance(crop, x, y)
			det := g8 / 255.0
			score.Skin += r8 / 255.0 * (det + skinBias) * imp
			score.Detail += det * imp
			score.Saturation += b8 / 255.0 * (det + saturationBias) * imp
		}
	}

	score.Total = (score.Detail*detailWeight + score.Skin*skinWeight + score.Saturation*saturationWeight) / float64(crop.Width) / float64(crop.Height)
	return score
}

func drawDebugCrop(topCrop cropInfo, o *image.RGBA) {
	w := o.Bounds().Size().X
	h := o.Bounds().Size().Y

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {

			r, g, b, _ := o.At(x, y).RGBA()
			r8 := float64(r >> 8)
			g8 := float64(g >> 8)
			b8 := uint8(b >> 8)

			imp := importance(topCrop, x, y)

			if imp > 0 {
				g8 += imp * 32
			} else if imp < 0 {
				r8 += imp * -64
			}

			nc := color.RGBA{uint8(bounds(r8)), uint8(bounds(g8)), b8, 255}
			o.SetRGBA(x, y, nc)
		}
	}
}

func analyse(settings CropSettings, img *image.RGBA, cropWidth, cropHeight, realMinScale float64) (image.Rectangle, error) {
	log := settings.Log
	o := image.NewRGBA(img.Bounds())

	now := time.Now()
	edgeDetect(img, o)
	log.Println("Time elapsed edge:", time.Since(now))
	debugOutput(settings.DebugMode, o, "edge")

	now = time.Now()
	skinDetect(img, o)
	log.Println("Time elapsed skin:", time.Since(now))
	debugOutput(settings.DebugMode, o, "skin")

	now = time.Now()
	saturationDetect(img, o)
	log.Println("Time elapsed sat:", time.Since(now))
	debugOutput(settings.DebugMode, o, "saturation")

	now = time.Now()
	var topCrop cropInfo
	topScore := -1.0
	cs := crops(o, cropWidth, cropHeight, realMinScale)
	log.Println("Time elapsed crops:", time.Since(now), len(cs))

	now = time.Now()
	for _, crop := range cs {
		nowIn := time.Now()
		crop.Score = score(o, crop)
		log.Println("Time elapsed single-score:", time.Since(nowIn))
		if crop.Score.Total > topScore {
			topCrop = crop
			topScore = crop.Score.Total
		}
	}
	log.Println("Time elapsed score:", time.Since(now))

	if settings.DebugMode {
		drawDebugCrop(topCrop, o)
		debugOutput(true, o, "final")
	}
	return image.Rect(topCrop.X, topCrop.Y, topCrop.X+topCrop.Width, topCrop.Y+topCrop.Height), nil
}

func saturation(c color.RGBA) float64 {
	cMax, cMin := uint8(0), uint8(255)
	if c.R > cMax {
		cMax = c.R
	}
	if c.R < cMin {
		cMin = c.R
	}
	if c.G > cMax {
		cMax = c.G
	}
	if c.G < cMin {
		cMin = c.G
	}
	if c.B > cMax {
		cMax = c.B
	}
	if c.B < cMin {
		cMin = c.B
	}

	if cMax == cMin {
		return 0
	}
	maximum := float64(cMax) / 255.0
	minimum := float64(cMin) / 255.0

	l := (maximum + minimum) / 2.0
	d := maximum - minimum

	if l > 0.5 {
		return d / (2.0 - maximum - minimum)
	}

	return d / (maximum + minimum)
}

func cie(c color.RGBA) float64 {
	return 0.5126*float64(c.B) + 0.7152*float64(c.G) + 0.0722*float64(c.R)
}

func skinCol(c color.RGBA) float64 {
	r8, g8, b8 := float64(c.R), float64(c.G), float64(c.B)

	mag := math.Sqrt(r8*r8 + g8*g8 + b8*b8)
	rd := r8/mag - skinColor[0]
	gd := g8/mag - skinColor[1]
	bd := b8/mag - skinColor[2]

	d := math.Sqrt(rd*rd + gd*gd + bd*bd)
	return 1.0 - d
}

func makeCies(img *image.RGBA) []float64 {
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()
	cies := make([]float64, h*w, h*w)
	i := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cies[i] = cie(img.RGBAAt(x, y))
			i++
		}
	}

	return cies
}

func edgeDetect(i *image.RGBA, o *image.RGBA) {
	w := i.Bounds().Dx()
	h := i.Bounds().Dy()
	cies := makeCies(i)

	var lightness float64
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {

			if x == 0 || x >= w-1 || y == 0 || y >= h-1 {
				//lightness = cie((*i).At(x, y))
				lightness = 0
			} else {
				lightness = cies[y*w+x]*4.0 -
					cies[x+(y-1)*w] -
					cies[x-1+y*w] -
					cies[x+1+y*w] -
					cies[x+(y+1)*w]
			}

			nc := color.RGBA{0, uint8(bounds(lightness)), 0, 255}
			o.SetRGBA(x, y, nc)
		}
	}
}

func skinDetect(i *image.RGBA, o *image.RGBA) {
	w := i.Bounds().Dx()
	h := i.Bounds().Dy()

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			lightness := cie(i.RGBAAt(x, y)) / 255.0
			skin := skinCol(i.RGBAAt(x, y))

			c := o.RGBAAt(x, y)
			if skin > skinThreshold && lightness >= skinBrightnessMin && lightness <= skinBrightnessMax {
				r := (skin - skinThreshold) * (255.0 / (1.0 - skinThreshold))
				nc := color.RGBA{uint8(bounds(r)), c.G, c.B, 255}
				o.SetRGBA(x, y, nc)
			} else {
				nc := color.RGBA{0, c.G, c.B, 255}
				o.SetRGBA(x, y, nc)
			}
		}
	}
}

func saturationDetect(i *image.RGBA, o *image.RGBA) {
	w := i.Bounds().Dx()
	h := i.Bounds().Dy()

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			lightness := cie(i.RGBAAt(x, y)) / 255.0
			saturation := saturation(i.RGBAAt(x, y))

			c := o.RGBAAt(x, y)
			if saturation > saturationThreshold && lightness >= saturationBrightnessMin && lightness <= saturationBrightnessMax {
				b := (saturation - saturationThreshold) * (255.0 / (1.0 - saturationThreshold))
				nc := color.RGBA{c.R, c.G, uint8(bounds(b)), 255}
				o.SetRGBA(x, y, nc)
			} else {
				nc := color.RGBA{c.R, c.G, 0, 255}
				o.SetRGBA(x, y, nc)
			}
		}
	}
}

func crops(i *image.RGBA, cropWidth, cropHeight, realMinScale float64) []cropInfo {
	res := []cropInfo{}
	width := i.Bounds().Dx()
	height := i.Bounds().Dy()

	minDimension := math.Min(float64(width), float64(height))
	var cropW, cropH float64

	if cropWidth != 0.0 {
		cropW = cropWidth
	} else {
		cropW = minDimension
	}
	if cropHeight != 0.0 {
		cropH = cropHeight
	} else {
		cropH = minDimension
	}

	for scale := maxScale; scale >= realMinScale; scale -= scaleStep {
		for y := 0; float64(y)+cropH*scale <= float64(height); y += step {
			for x := 0; float64(x)+cropW*scale <= float64(width); x += step {
				res = append(res, cropInfo{
					X:      x,
					Y:      y,
					Width:  int(cropW * scale),
					Height: int(cropH * scale),
				})
			}
		}
	}

	return res
}

func toRGBA(img image.Image) *image.RGBA {
	switch img.(type) {
	case *image.RGBA:
		return img.(*image.RGBA)
	}
	out := image.NewRGBA(img.Bounds())
	draw.Copy(out, image.Pt(0, 0), img, img.Bounds(), draw.Src, nil)
	return out
}
