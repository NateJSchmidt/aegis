# Aegis

[Historical context](https://www.dictionary.com/browse/aegis)

[Ship-related context](https://www.navy.mil/Resources/Fact-Files/Display-FactFiles/Article/2166739/aegis-weapon-system/#:~:text=Description,%2Dfunction%20phased%2Darray%20radar.)

## Runtime configuration

There are a number of configs that are hard coded at the moment.  List is provided here:

* logic to determine which monitor to screen capture
* rectangular coordinates to capture on said screen
* the colors yellow, orange, red that correlate to neutrals, bads, and terribles (standing ratings)
* cycle time of your mining laser

## Required config (in game)

1. You **must** disable window transparency (otherwise Aegis is unnable to correctly detect colors)
  * Escape -> General -> Window Appearance -> slide both sliders all the way to the left
1. You **must** set `neutral` and `no standing` colors to yellow (same reason)
  * Undock -> Overview settings -> Appearance -> bottom list, left click to set color for:  `Pilot has Neutral Standing` and `Pilot has No Standing`

## Build instructions

### Windows

In the top directory, run `go build -o aegis.exe cmd/main.go`.  Note that all of the audio files are compiled into the final executable.

### Nate's Config

#### 3 monitor

(-2195, 569) to (-2020, 1309)

#### 2 monitor

(-2315, 1131) to (-2195, 1631)
