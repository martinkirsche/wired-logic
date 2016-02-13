Wired Logic
===========

Wired Logic can turn a still image like this…

![input image](examples/input.gif)

…into an animation like that…

![output image](examples/output.gif)

How to run it?
--------------

Execute the following command to convert a static image into an animation…

    go run $GOPATH/src/github.com/martinkirsche/wired-logic/apps/gif/main.go input.gif output.gif

…or draw your own circuit using the [Wired Logic Sandbox](http://martinkirsche.github.com/wired-logic/) in your browser.

How does it work?
-----------------

It scans the image, converts it into a collection of wires, power sources and
transistors and runs a simulation on them as long as the state of the 
simulation does not recur. Then it renders the simulation into the animated
gif image.

### The rules

Description | Example  
------------|--------
Wires are all pixels of the color from index 1 to 7 within the palette. | ![wire](examples/wire.gif) 
A 2x2 pixel square within a wire will make the wire a power source. | ![wire](examples/source.gif)
Wires can cross each other by poking a hole in the middle of their crossing. | ![wire](examples/crossing.gif
A transistor gets created by drawing an arbitrarily rotated T-shape and, you guessed it, poking a hole in the middle of their crossing. If a transistor's base gets charged it will stop current from flowing. If not, current will flow but gets reduced by one. | ![wire](examples/transistor.gif)

The idea
--------

Wired Logic was mainly inspired by Minecraft's Redstone and [Wireworld]. The first prototype even was a cellular automaton like [Wireworld] running as a shader within the GPU where each pixel passed its `charge - 1` on to its neighbours. But it was slow and impractical so I came up with this implementation. 

[Wireworld]: https://en.wikipedia.org/wiki/Wireworld