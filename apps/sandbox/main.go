package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"log"
	"os"

	"github.com/hajimehoshi/ebiten"
	"github.com/martinkirsche/wired-logic/simulation"
)

var simulationImage *image.Paletted
var currentSimulation *simulation.Simulation
var backgroundImage *ebiten.Image
var oldCursorPosition image.Point = image.Point{-1, -1}
var wireImages []*ebiten.Image
var cursorBlinking uint8
var cursorImage *ebiten.Image

func main() {
	var err error
	if cursorImage, err = ebiten.NewImage(4, 4, ebiten.FilterNearest); err != nil {
		log.Fatal(err)
	}
	cursorImage.Fill(color.White)
	var scale, width, height int
	flag.IntVar(&scale, "scale", 16, "pixel scale factor")
	flag.IntVar(&width, "width", 64, "width of the simulation")
	flag.IntVar(&height, "height", 64, "height of the simulation")
	flag.Parse()
	flag.Args()

	if flag.NArg() == 1 {
		inputFileName := flag.Arg(0)
		in, err := os.Open(inputFileName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		gifImage, err := gif.DecodeAll(in)
		if err != nil {
			log.Fatal(err)
		}
		simulationImage = gifImage.Image[0]
		simulationImage.Palette[0] = color.Transparent
	} else {
		p := color.Palette{
			color.Black,
			color.RGBA{0x88, 0, 0, 0xFF},
			color.RGBA{0xFF, 0, 0, 0xFF},
			color.RGBA{0xFF, 0x22, 0, 0xFF},
			color.RGBA{0xFF, 0x44, 0, 0xFF},
			color.RGBA{0xFF, 0x66, 0, 0xFF},
			color.RGBA{0xFF, 0x88, 0, 0xFF},
			color.RGBA{0xFF, 0xAA, 0, 0xFF},
		}
		simulationImage = image.NewPaletted(image.Rect(0, 0, width, height), p)
	}
	reloadSimulation()
	if err := ebiten.Run(update, simulationImage.Bounds().Dx(), simulationImage.Bounds().Dy(), scale, "Wired Logic"); err != nil {
		log.Fatal(err)
	}
}

func reloadSimulation() error {
	currentSimulation = simulation.New(simulationImage)
	currentSimulation.Draw(simulationImage)
	var err error
	backgroundImage, err = ebiten.NewImageFromImage(simulationImage, ebiten.FilterNearest)
	if err != nil {
		log.Fatal(err)
	}
	for _, img := range wireImages {
		if err = img.Dispose(); err != nil {
			return err
		}
	}
	wires := currentSimulation.Circuit().Wires()
	wireImages = make([]*ebiten.Image, len(wires))
	for i, wire := range wires {
		img := drawMask(wire)
		var err error
		if wireImages[i], err = ebiten.NewImageFromImage(img, ebiten.FilterNearest); err != nil {
			return err
		}
	}
	return nil
}

func togglePixel(position image.Point) error {
	currentSimulation.Draw(simulationImage)
	c := simulationImage.ColorIndexAt(position.X, position.Y)
	if c-1 > simulation.MaxCharge {
		simulationImage.SetColorIndex(position.X, position.Y, 1)
	} else {
		simulationImage.SetColorIndex(position.X, position.Y, 0)
	}
	if err := reloadSimulation(); err != nil {
		return err
	}
	return nil
}

func handleCursor(screen *ebiten.Image) error {
	mx, my := ebiten.CursorPosition()
	if cursorBlinking == 127 {
		cursorBlinking = 0
	} else {
		cursorBlinking++
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(0.25, .25)
	op.GeoM.Translate(float64(mx), float64(my))
	if cursorBlinking > 64 {
		op.ColorM.Scale(1, 1, 1, 0.25+float64(127-cursorBlinking)/255.0)
	} else {
		op.ColorM.Scale(1, 1, 1, 0.25+float64(cursorBlinking)/255.0)
	}
	if err := screen.DrawImage(cursorImage, op); err != nil {
		return err
	}
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		if mx != oldCursorPosition.X || my != oldCursorPosition.Y {
			oldCursorPosition = image.Point{mx, my}
			if err := togglePixel(oldCursorPosition); err != nil {
				return err
			}
		}
	} else {
		oldCursorPosition = image.Point{-1, -1}
	}
	return nil
}

func update(screen *ebiten.Image) error {
	newSimulation := currentSimulation.Step()
	wires := currentSimulation.Circuit().Wires()
	for i, wire := range wires {
		oldCharge := currentSimulation.State(wire).Charge()
		charge := newSimulation.State(wire).Charge()
		if oldCharge == charge {
			continue
		}
		position := wire.Bounds().Min
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(position.X), float64(position.Y))
		r, g, b, a := simulationImage.Palette[charge+1].RGBA()
		op.ColorM.Scale(float64(r)/0xFFFF, float64(g)/0xFFFF, float64(b)/0xFFFF, float64(a)/0xFFFF)
		var err error
		if err = backgroundImage.DrawImage(wireImages[i], op); err != nil {
			return err
		}
	}
	currentSimulation = newSimulation
	if err := screen.DrawImage(backgroundImage, &ebiten.DrawImageOptions{}); err != nil {
		return err
	}

	if err := handleCursor(screen); err != nil {
		return err
	}
	return nil
}

func drawMask(wire *simulation.Wire) image.Image {
	bounds := image.Rect(0, 0, wire.Bounds().Dx(), wire.Bounds().Dy())
	bounds = bounds.Union(image.Rect(0, 0, 4, 4))
	position := wire.Bounds().Min
	img := image.NewRGBA(bounds)
	white := color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
	for _, pixel := range wire.Pixels() {
		img.SetRGBA(pixel.X-position.X, pixel.Y-position.Y, white)
	}
	return img
}
