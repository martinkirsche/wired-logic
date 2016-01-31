package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"image"
	"image/gif"
	"log"
	"os"
)

const maxCharge = 6

func main() {

	inputFileName := os.Args[1]
	outputFileName := os.Args[2]

	in, err := os.Open(inputFileName)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	gifImage, err := gif.DecodeAll(in)
	if err != nil {
		log.Fatal(err)
	}

	gifImage.Image = gifImage.Image[:1]
	gifImage.Delay = gifImage.Delay[:1]
	gifImage.Disposal = gifImage.Disposal[:1]

	img := gifImage.Image[0]

	circuit := NewCircuit(img)

	transparentColorIndex := uint8(0)
	for index, color := range img.Palette {
		if _, _, _, a := color.RGBA(); a != 0 {
			continue
		}
		transparentColorIndex = uint8(index)
		break
	}

	loopingHash := circuit.FindLoopingHash()
	circuit.Simulate()
	circuit.Draw(gifImage.Image[0])
	for !bytes.Equal(loopingHash[:], circuit.Hash()) {
		img := image.NewPaletted(img.Bounds(), img.Palette)
		palettedFill(img, transparentColorIndex)
		circuit.Simulate()
		circuit.DiffDraw(img)
		gifImage.Image = append(gifImage.Image, img)
		gifImage.Delay = append(gifImage.Delay, 1)
		gifImage.Disposal = append(gifImage.Disposal, 0)
	}

	out, err := os.Create(outputFileName)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = gif.EncodeAll(out, gifImage)
	if err != nil {
		log.Fatal(err)
	}
}

func palettedFill(img *image.Paletted, index uint8) {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.SetColorIndex(x, y, index)
		}
	}
}

type Circuit struct {
	wires       []*wire
	transistors []*transistor
}

func NewCircuit(img *image.Paletted) Circuit {
	size := img.Bounds().Size()
	groups := make(map[*group]struct{}, 0)
	matrix := newBucketMatrix(size.X, size.Y)
	for y := 0; y < size.Y; y++ {
		for x := 0; x < size.X; x++ {
			r := img.ColorIndexAt(x, y)
			if r <= maxCharge {
				topLeftBucket := matrix.get(x-1, y-1)
				topBucket := matrix.get(x, y-1)
				leftBucket := matrix.get(x-1, y)
				var currentBucket *bucket
				if nil == topBucket && nil == leftBucket {
					currentBucket = newBucket()
					groups[currentBucket.group] = struct{}{}
				} else if nil == topBucket && nil != leftBucket {
					currentBucket = leftBucket
				} else if (nil != topBucket && nil == leftBucket) ||
					topBucket == leftBucket ||
					topBucket.group == leftBucket.group {
					currentBucket = topBucket
				} else {
					currentBucket = topBucket
					delete(groups, topBucket.group)
					topBucket.group.moveBucketsTo(leftBucket.group)
				}
				if nil != topLeftBucket && nil != topBucket && nil != leftBucket {
					currentBucket.group.wire.isPowerSource = true
					currentBucket.group.wire.charge = maxCharge
					currentBucket.group.wire.previousCharge = maxCharge
				}
				matrix.set(x, y, currentBucket)
				currentBucket.addPixel(image.Point{x, y})
				if r > currentBucket.group.wire.charge {
					currentBucket.group.wire.charge = r
					currentBucket.group.wire.previousCharge = r
				}
			}
		}
	}

	for y := 0; y < size.Y; y++ {
		for x := 0; x < size.X; x++ {
			if nil != matrix.get(x, y) {
				continue
			}
			topBucket := matrix.get(x, y-1)
			topRightBucket := matrix.get(x+1, y-1)
			rightBucket := matrix.get(x+1, y)
			bottomRightBucket := matrix.get(x+1, y+1)
			bottomBucket := matrix.get(x, y+1)
			bottomLeftBucket := matrix.get(x-1, y+1)
			leftBucket := matrix.get(x-1, y)
			topLeftBucket := matrix.get(x-1, y-1)
			if nil == topLeftBucket && nil == topRightBucket && nil == bottomLeftBucket && nil == bottomRightBucket &&
				nil != topBucket && nil != rightBucket && nil != bottomBucket && nil != leftBucket {
				delete(groups, topBucket.group)
				topBucket.group.moveBucketsTo(bottomBucket.group)
				delete(groups, leftBucket.group)
				leftBucket.group.moveBucketsTo(rightBucket.group)
			}
		}
	}

	transistors := make([]*transistor, 0)
	for y := 0; y < size.Y; y++ {
		for x := 0; x < size.X; x++ {
			if nil != matrix.get(x, y) {
				continue
			}
			topBucket := matrix.get(x, y-1)
			topRightBucket := matrix.get(x+1, y-1)
			rightBucket := matrix.get(x+1, y)
			bottomRightBucket := matrix.get(x+1, y+1)
			bottomBucket := matrix.get(x, y+1)
			bottomLeftBucket := matrix.get(x-1, y+1)
			leftBucket := matrix.get(x-1, y)
			topLeftBucket := matrix.get(x-1, y-1)

			switch {
			case nil == bottomLeftBucket && nil == bottomRightBucket &&
				nil == topBucket && nil != rightBucket && nil != bottomBucket && nil != leftBucket:
				transistors = append(transistors,
					newTransistor(image.Point{x, y}, bottomBucket.group.wire, rightBucket.group.wire, leftBucket.group.wire))
			case nil == bottomLeftBucket && nil == topLeftBucket &&
				nil != topBucket && nil == rightBucket && nil != bottomBucket && nil != leftBucket:
				transistors = append(transistors,
					newTransistor(image.Point{x, y}, leftBucket.group.wire, topBucket.group.wire, bottomBucket.group.wire))
			case nil == topLeftBucket && nil == topRightBucket &&
				nil != topBucket && nil != rightBucket && nil == bottomBucket && nil != leftBucket:
				transistors = append(transistors,
					newTransistor(image.Point{x, y}, topBucket.group.wire, rightBucket.group.wire, leftBucket.group.wire))
			case nil == bottomRightBucket && nil == topRightBucket &&
				nil != topBucket && nil != rightBucket && nil != bottomBucket && nil == leftBucket:
				transistors = append(transistors,
					newTransistor(image.Point{x, y}, rightBucket.group.wire, topBucket.group.wire, bottomBucket.group.wire))
			}
		}
	}

	wires := make([]*wire, len(groups))
	i := 0
	for k := range groups {
		wires[i] = k.wire
		i++
	}

	return Circuit{wires: wires, transistors: transistors}
}

func (c *Circuit) Simulate() {
	for _, wire := range c.wires {
		if wire.isPowerSource {
			continue
		}
		wire.previousCharge = wire.charge
	}
	for _, wire := range c.wires {
		if wire.isPowerSource {
			continue
		}
		source := wire.tracePowerSource()
		if source.previousCharge > wire.previousCharge+1 {
			wire.charge++
		} else if source.previousCharge <= wire.previousCharge && wire.previousCharge > 0 {
			wire.charge--
		}
	}
}

func (c *Circuit) DiffDraw(img *image.Paletted) {
	for _, wire := range c.wires {
		if wire.previousCharge == wire.charge {
			continue
		}
		wire.draw(img, wire.charge)
	}
}

func (c *Circuit) Draw(img *image.Paletted) {
	for _, wire := range c.wires {
		wire.draw(img, wire.charge)
	}
	for _, transistor := range c.transistors {
		transistor.draw(img, 9)
	}
}

func (c *Circuit) FindLoopingHash() []byte {
	hashs := make(map[[sha1.Size]byte]struct{}, 0)
	for {
		c.Simulate()
		var hash [sha1.Size]byte
		copy(hash[:], c.Hash())
		if _, ok := hashs[hash]; ok {
			return hash[:]
		}
		hashs[hash] = struct{}{}
	}
}

func (c *Circuit) Hash() []byte {
	hash := sha1.New()
	for index, wire := range c.wires {
		buf := new(bytes.Buffer)

		err := binary.Write(buf, binary.LittleEndian, uint32(index))
		if err != nil {
			log.Fatal(err)
		}
		err = binary.Write(buf, binary.LittleEndian, wire.charge)
		if err != nil {
			log.Fatal(err)
		}
		_, err = hash.Write(buf.Bytes())
		if err != nil {
			log.Fatal(err)
		}
	}
	return hash.Sum(nil)
}

type transistor struct {
	position image.Point
	base     *wire
	inputA   *wire
	inputB   *wire
}

func newTransistor(position image.Point, base, inputA, inputB *wire) *transistor {
	transistor := &transistor{
		position: position,
		base:     base,
		inputA:   inputA,
		inputB:   inputB,
	}
	inputA.transistors = append(inputA.transistors, transistor)
	inputB.transistors = append(inputB.transistors, transistor)
	return transistor
}

func (t *transistor) draw(img *image.Paletted, colorIndex uint8) {
	img.SetColorIndex(t.position.X, t.position.Y, colorIndex)
}

type wire struct {
	pixels         []image.Point
	bounds         image.Rectangle
	transistors    []*transistor
	isPowerSource  bool
	charge         uint8
	previousCharge uint8
}

func newWire() *wire {
	return &wire{
		pixels:         make([]image.Point, 0),
		bounds:         image.Rectangle{image.Pt(0, 0), image.Pt(0, 0)},
		transistors:    make([]*transistor, 0),
		isPowerSource:  false,
		charge:         0,
		previousCharge: 0,
	}
}

func (w *wire) draw(img *image.Paletted, colorIndex uint8) {
	for _, pixel := range w.pixels {
		img.SetColorIndex(pixel.X, pixel.Y, colorIndex)
	}
}

func (w *wire) tracePowerSource() *wire {
	result := w
	for _, transistor := range w.transistors {
		if nil != transistor.base && transistor.base.previousCharge > 0 {
			continue
		}
		if w == transistor.inputA {
			if transistor.inputB.isPowerSource {
				return transistor.inputB
			}
			if transistor.inputB.previousCharge > result.previousCharge {
				result = transistor.inputB
				continue
			}
		} else if w == transistor.inputB {
			if transistor.inputA.isPowerSource {
				return transistor.inputA
			}
			if transistor.inputA.previousCharge > result.previousCharge {
				result = transistor.inputA
				continue
			}
		}
	}
	return result
}

type bucketMatrix struct {
	buckets [][]*bucket
	width   int
	height  int
}

func newBucketMatrix(width int, height int) *bucketMatrix {
	m := &bucketMatrix{make([][]*bucket, height), width, height}
	for y := 0; y < height; y++ {
		m.buckets[y] = make([]*bucket, width)
	}
	return m
}

func (m *bucketMatrix) get(x int, y int) *bucket {
	if x < 0 || y < 0 || x >= m.width || y >= m.height {
		return nil
	}
	return m.buckets[y][x]
}

func (m *bucketMatrix) set(x int, y int, bucket *bucket) {
	m.buckets[y][x] = bucket
}

type bucket struct {
	group *group
}

func newBucket() *bucket {

	newBucket := &bucket{nil}
	newGroup := &group{
		buckets: []*bucket{newBucket},
		wire:    newWire(),
	}
	newBucket.group = newGroup
	return newBucket
}

func (b *bucket) addPixel(pixel image.Point) {
	b.group.wire.pixels = append(b.group.wire.pixels, pixel)
	b.group.wire.bounds = b.group.wire.bounds.Union(
		image.Rectangle{
			pixel,
			pixel.Add(image.Point{1, 1})})
}

type group struct {
	buckets []*bucket
	wire    *wire
}

func (g *group) moveBucketsTo(other *group) {
	for _, bucket := range g.buckets {
		bucket.group = other
		other.buckets = append(other.buckets, bucket)
	}
	if g.wire.isPowerSource {
		other.wire.isPowerSource = true
		other.wire.charge = maxCharge
		other.wire.previousCharge = maxCharge
	}
	other.wire.bounds = other.wire.bounds.Union(g.wire.bounds)
	other.wire.pixels = append(other.wire.pixels, g.wire.pixels...)
}
