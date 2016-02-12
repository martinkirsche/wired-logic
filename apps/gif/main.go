package main

import (
	"flag"
	"fmt"
	"image/color"
	"image/gif"
	"log"
	"os"

	"github.com/martinkirsche/wired-logic/simulation"
)

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
	sim := simulation.New(img)

	log.Println("simulating...")

	for ; startFrame > 0; startFrame-- {
		sim = sim.Step()
	}

	if 0 == frameCount {
		sim, frameCount = sim.FindLooping()
	}

	log.Println("rendering...")
	img.Palette[0] = color.Transparent

	gifImage.Delay = make([]int, frameCount)
	gifImage.Disposal = make([]byte, frameCount)
	for i := range gifImage.Delay {
		gifImage.Delay[i] = 1
	}
	gifImage.Image = sim.DrawAll(img, frameCount)

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
