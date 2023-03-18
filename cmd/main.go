package main

import (
	"embed"
	"fmt"
	"image"
	"image/png"
	"os"
	"sync"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
	"github.com/kbinani/screenshot"
)

//go:embed audio/*
var audioFiles embed.FS

func main() {
	fmt.Println("This program runs indefintely, ctrl+c to exit.")

	captureLeftScreen()

	quit := make(chan bool)
	var lock sync.Mutex

	// start up the timer loop
	go timerLoop(quit, &lock)

	fmt.Println("Starting main loop")
	for {
		img := captureScreen(&lock)

		foundBaddie := checkPixels(img)

		if foundBaddie {
			playChime(&lock)
			break
		} else {
			time.Sleep(1 * time.Second)
		}

	}

	fmt.Println("Sending signal")
	quit <- true
	fmt.Println("Ending program")
	// need to exit here to stop all goroutines
}

func timerLoop(quit <-chan bool, lock *sync.Mutex) {
	fmt.Println("Starting cycle timer noises")

	for {
		// sleep first, then handle the signal and/or play noise
		// time.Sleep((93600 - 1226) * 2 * time.Millisecond)
		time.Sleep(3 * time.Second)

		select {
		case <-quit:
			fmt.Println("Ending cycle timer noises")
			return
		default:
			playChimes(lock)
		}
	}
}

func checkPixels(img *image.RGBA) bool {
	retval := false
	for x := img.Rect.Min.X; x <= img.Rect.Max.X; x++ {
		for y := img.Rect.Min.Y; y <= img.Rect.Max.Y; y++ {
			color := img.RGBAAt(x, y)

			if color.R == 117 && color.G == 10 && color.B == 10 {
				// color is red, play chime
				fmt.Println("Found red")
				retval = true
				break
			} else if color.R == 153 && color.G == 60 && color.B == 10 {
				// color is orange play chime
				fmt.Println("Found orange")
				retval = true
				break
			} else if color.R == 153 && color.G == 110 && color.B == 10 {
				// color is yellow, play chime
				fmt.Println("Found yellow")
				retval = true
				break
			} else {
				// fmt.Println("Found nothing")
			}
		}
		if retval {
			break
		}
	}
	return retval
}

func captureScreen(lock *sync.Mutex) *image.RGBA {
	n := screenshot.NumActiveDisplays()
	// fmt.Printf("Number of displays: %d\n", n)

	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)
		if bounds.Min.X < 0 {
			// img, err := screenshot.CaptureRect(bounds)

			//when in 3 screen mode
			//357,609 - 510, 1333
			img, err := screenshot.CaptureRect(image.Rect(bounds.Min.X+357, bounds.Min.Y+609, bounds.Min.X+510, bounds.Min.Y+1333))

			//when in 2 screen mode
			//220,867 - 340,1367
			// img, err := screenshot.CaptureRect(image.Rect(bounds.Min.X+220, bounds.Min.Y+867, bounds.Min.X+340, bounds.Min.Y+1367))
			if err != nil {
				fmt.Printf("Failure occurred: %s\n", err)
				playCrashNoise(lock)
				panic(err)
			}

			// filename := fmt.Sprintf("%d_%dx%d.png", i, bounds.Dx(), bounds.Dy())
			// file, _ := os.Create(filename)
			// defer file.Close()
			// png.Encode(file, img)

			// fmt.Printf("#%d : %v \"%s\"\n", i, bounds, filename)

			// panic("arrrr")

			return img
		}
	}
	return nil
}

func captureLeftScreen() {
	n := screenshot.NumActiveDisplays()
	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)
		if bounds.Min.X < 0 {
			// img, err := screenshot.CaptureRect(bounds)

			//when in 3 screen mode
			//357,609 - 510, 1333
			img, err := screenshot.CaptureRect(image.Rect(bounds.Min.X+357, bounds.Min.Y+609, bounds.Min.X+510, bounds.Min.Y+1333))

			//when in 2 screen mode
			//220,867 - 340,1367
			// img, err := screenshot.CaptureRect(image.Rect(bounds.Min.X+220, bounds.Min.Y+867, bounds.Min.X+340, bounds.Min.Y+1367))
			if err != nil {
				fmt.Printf("Failure occurred: %s\n", err)
				panic(err)
			}

			filename := fmt.Sprintf("%d_%dx%d.png", i, bounds.Dx(), bounds.Dy())
			file, _ := os.Create(filename)
			defer file.Close()
			png.Encode(file, img)

			fmt.Printf("#%d : %v \"%s\"\n", i, bounds, filename)
		}
	}
}

func playChime(lock *sync.Mutex) {
	f, err := audioFiles.Open("audio/chime-sound-7143.mp3")
	if err != nil {
		fmt.Printf("Failure occurred: %s\n", err)
		panic(err)
	}
	defer f.Close()

	streamer, format, err := mp3.Decode(f)
	if err != nil {
		fmt.Printf("Failure occurred: %s\n", err)
		panic(err)
	}
	defer streamer.Close()

	lock.Lock()
	defer lock.Unlock()
	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

	done := make(chan bool)
	speaker.Play(beep.Seq(streamer, beep.Callback(func() { done <- true })))
	<-done
}

func playCrashNoise(lock *sync.Mutex) {
	f, err := audioFiles.Open("audio/WindowsHardwareFail_amplified.wav")
	if err != nil {
		fmt.Printf("Failure occurred: %s\n", err)
		panic(err)
	}
	defer f.Close()

	streamer, format, err := wav.Decode(f)
	if err != nil {
		fmt.Printf("Failure occurred: %s\n", err)
		panic(err)
	}
	defer streamer.Close()

	lock.Lock()
	defer lock.Unlock()
	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

	done := make(chan bool)
	speaker.Play(beep.Seq(streamer, beep.Callback(func() { done <- true })))
	<-done
}

func playChimes(lock *sync.Mutex) {
	f, err := audioFiles.Open("audio/chimes_amplified.wav")
	if err != nil {
		fmt.Printf("Failure occurred: %s\n", err)
		panic(err)
	}
	defer f.Close()

	streamer, format, err := wav.Decode(f)
	if err != nil {
		fmt.Printf("Failure occurred: %s\n", err)
		panic(err)
	}
	defer streamer.Close()

	lock.Lock()
	defer lock.Unlock()
	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

	done := make(chan bool)
	speaker.Play(beep.Seq(streamer, beep.Callback(func() { done <- true })))
	<-done
}
