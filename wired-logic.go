package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"log"
	"os"
)

const maxCharge = 6

func main() {

	var startFrame int
	var frameCount int
	flag.IntVar(&startFrame, "start", 0, "frame at wich the animation should start")
	flag.IntVar(&frameCount, "count", 0, "amount of frames the animation should have")
	flag.Parse()
	inputFileName := flag.Arg(0)
	outputFileName := flag.Arg(1)

	in, err := os.Open(inputFileName)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	gifImage, err := gif.DecodeAll(in)
	if err != nil {
		log.Fatal(err)
	}

	img := gifImage.Image[0]

	log.Println("converting...")
	simulation := NewSimulation(img)

	log.Println("simulating...")

	for ; startFrame > 0; startFrame-- {
		simulation = simulation.Step()
	}

	if 0 == frameCount {
		simulation, frameCount = simulation.FindLooping()
	}

	log.Println("rendering...")
	img.Palette[0] = color.Transparent

	gifImage.Delay = make([]int, frameCount)
	gifImage.Disposal = make([]byte, frameCount)
	for i := range gifImage.Delay {
		gifImage.Delay[i] = 1
	}
	gifImage.Image = simulation.DrawAll(img, frameCount)

	log.Println("writing...")
	out, err := os.Create(outputFileName)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = gif.EncodeAll(out, gifImage)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("done.")
}

type circuit struct {
	wires       []*wire
	transistors []*transistor
}

type wireState struct {
	charge uint8
	wire   *wire
}

type Simulation struct {
	circuit *circuit
	states  []wireState
}

func NewSimulation(img *image.Paletted) *Simulation {
	size := img.Bounds().Size()
	groups := make(map[*group]struct{}, 0)
	matrix := newBucketMatrix(size.X, size.Y)
	for y := 0; y < size.Y; y++ {
		for x := 0; x < size.X; x++ {
			charge := img.ColorIndexAt(x, y) - 1
			if charge > maxCharge {
				continue
			}
			topLeftBucket := matrix.get(x-1, y-1)
			topBucket := matrix.get(x, y-1)
			leftBucket := matrix.get(x-1, y)
			var currentBucket *bucket
			switch {
			case nil == topBucket && nil == leftBucket:
				currentBucket = newBucket()
				groups[currentBucket.group] = struct{}{}
			case nil == topBucket && nil != leftBucket:
				currentBucket = leftBucket
			case (nil != topBucket && nil == leftBucket) ||
				topBucket == leftBucket ||
				topBucket.group == leftBucket.group:
				currentBucket = topBucket
			default:
				currentBucket = topBucket
				delete(groups, topBucket.group)
				topBucket.group.moveBucketsTo(leftBucket.group)
			}
			if nil != topLeftBucket && nil != topBucket && nil != leftBucket {
				currentBucket.group.wire.isPowerSource = true
			}
			matrix.set(x, y, currentBucket)
			if charge > currentBucket.group.wireState.charge {
				currentBucket.group.wireState.charge = charge
			}
			currentBucket.addPixel(image.Point{x, y})
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
				if topBucket.group != bottomBucket.group {
					delete(groups, topBucket.group)
					topBucket.group.moveBucketsTo(bottomBucket.group)
				}
				if rightBucket.group != leftBucket.group {
					delete(groups, rightBucket.group)
					rightBucket.group.moveBucketsTo(leftBucket.group)
				}
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
	wireStates := make([]wireState, len(groups))
	i := 0
	for k := range groups {
		k.wire.index = i
		wires[i] = k.wire
		wireStates[i] = k.wireState
		i++
	}

	return &Simulation{&circuit{wires: wires, transistors: transistors}, wireStates}
}

func (s *Simulation) Step() *Simulation {
	newWireState := make([]wireState, len(s.states))
	for i, state := range s.states {
		charge := state.charge
		if state.wire.isPowerSource {
			if state.charge < maxCharge {
				charge = state.charge + 1
			}
		} else {
			source := s.tracePowerSource(state)
			if source.charge > state.charge+1 {
				charge = state.charge + 1
			} else if source.charge <= state.charge && state.charge > 0 {
				charge = state.charge - 1
			}
		}
		newWireState[i] = wireState{charge, state.wire}
	}
	return &Simulation{s.circuit, newWireState}
}

func (s *Simulation) tracePowerSource(origin wireState) wireState {
	result := origin
	for _, transistor := range origin.wire.transistors {
		if nil != transistor.base && s.states[transistor.base.index].charge > 0 {
			continue
		}
		if origin.wire == transistor.inputA {
			inputBState := s.states[transistor.inputB.index]
			if inputBState.charge == maxCharge {
				return inputBState
			}
			if inputBState.charge > result.charge {
				result = inputBState
				continue
			}
		} else if origin.wire == transistor.inputB {
			inputAState := s.states[transistor.inputA.index]
			if inputAState.charge == maxCharge {
				return inputAState
			}
			if inputAState.charge > result.charge {
				result = inputAState
				continue
			}
		}
	}
	return result
}

func (s *Simulation) DiffDraw(previousSimulation *Simulation, img *image.Paletted) {
	for i, state := range s.states {
		if previousSimulation.states[i].charge == state.charge {
			continue
		}
		state.wire.draw(img, state.charge+1)
	}
}

func (s *Simulation) Draw(img *image.Paletted) {
	for _, state := range s.states {
		state.wire.draw(img, state.charge+1)
	}
	for _, transistor := range s.circuit.transistors {
		transistor.draw(img, maxCharge+2)
	}
}

func (s *Simulation) DrawAll(initialImage *image.Paletted, frameCount int) []*image.Paletted {
	bounds := initialImage.Bounds()
	images := make([]*image.Paletted, frameCount)
	s.Draw(initialImage)
	images[0] = initialImage
	for f := 1; f < frameCount; f++ {
		newSimulation := s.Step()
		img := image.NewPaletted(bounds, initialImage.Palette)
		newSimulation.DiffDraw(s, img)
		images[f] = img
		s = newSimulation
	}
	return images
}

func (s *Simulation) FindLooping() (*Simulation, int) {
	hashs := make(map[[sha1.Size]byte]int, 0)
	frame := 0
	for {
		s = s.Step()
		var hash [sha1.Size]byte
		copy(hash[:], s.Hash())
		if f, ok := hashs[hash]; ok {
			return s, frame - f
		}
		hashs[hash] = frame
		frame++
	}
}

func (s *Simulation) Hash() []byte {
	hash := sha1.New()

	for index, state := range s.states {
		buf := new(bytes.Buffer)

		err := binary.Write(buf, binary.LittleEndian, uint32(index))
		if err != nil {
			log.Fatal(err)
		}
		err = binary.Write(buf, binary.LittleEndian, state.charge)
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
	index         int
	pixels        []image.Point
	bounds        image.Rectangle
	transistors   []*transistor
	isPowerSource bool
}

func newWire() *wire {
	return &wire{
		index:         -1,
		pixels:        make([]image.Point, 0),
		bounds:        image.Rectangle{image.Pt(0, 0), image.Pt(0, 0)},
		transistors:   make([]*transistor, 0),
		isPowerSource: false,
	}
}

func (w *wire) draw(img *image.Paletted, colorIndex uint8) {
	for _, pixel := range w.pixels {
		img.SetColorIndex(pixel.X, pixel.Y, colorIndex)
	}
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
	newGroup.wireState = wireState{wire: newGroup.wire, charge: 0}
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
	buckets   []*bucket
	wire      *wire
	wireState wireState
}

func (g *group) moveBucketsTo(other *group) {
	if g == other {
		log.Fatal("A group can not be moved to itself.")
	}
	for _, bucket := range g.buckets {
		bucket.group = other
		other.buckets = append(other.buckets, bucket)
	}
	if g.wire.isPowerSource {
		other.wire.isPowerSource = true
	}
	if g.wireState.charge > other.wireState.charge {
		other.wireState.charge = g.wireState.charge
	}
	other.wire.bounds = other.wire.bounds.Union(g.wire.bounds)
	other.wire.pixels = append(other.wire.pixels, g.wire.pixels...)
}
