package main

import (
	"bufio"
	"embed"
	"fmt"
	"github.com/schollz/progressbar"
	"image"
	"image/png"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
	"github.com/kbinani/screenshot"
	"gopkg.in/yaml.v3"
)

//go:embed audio/*
var audioFiles embed.FS

type YamlConfig struct {
	ScanConfigs  map[string]ScanConfig `yaml:"ScanConfigs"`
	ColorMatches []ColorMatch          `yaml:"ColorMatches"`
}

type ScanConfig struct {
	Monitor     int `yaml:"Monitor"`
	BottomLeftX int `yaml:"BottomLeftX"`
	BottomLeftY int `yaml:"BottomLeftY"`
	TopRightX   int `yaml:"TopRightX"`
	TopRightY   int `yaml:"TopRightY"`
}

type ColorMatch struct {
	MatchName string `yaml:"MatchName"`
	R         int    `yaml:"R"`
	G         int    `yaml:"G"`
	B         int    `yaml:"B"`
}

func (cm ColorMatch) hash() string {
	return strconv.Itoa(cm.R) + strconv.Itoa(cm.G) + strconv.Itoa(cm.B)
}

type ActiveConfig struct {
	ScanConfig    ScanConfig
	ColorMatchMap map[string]ColorMatch
}

func main() {
	fmt.Println("This program runs indefintely, ctrl+c to exit.")

	activeConfig := setup()

	captureLeftScreen(&activeConfig)

	quit := make(chan bool)
	var lock sync.Mutex

	// start up the timer loop
	// go timerLoop(quit, &lock)

	fmt.Println("Starting main loop")
	for {
		img := captureScreen(&lock, activeConfig)

		foundBaddie := checkPixels(img, activeConfig)

		if foundBaddie {
			playChime(&lock)
			waitForEnterKey()
		} else {
			time.Sleep(1 * time.Second)
		}
	}

	fmt.Println("Sending signal")
	quit <- true
	fmt.Println("Ending program")
	// need to exit here to stop all goroutines
}

func setup() (activeConfig ActiveConfig) {
	yamlConfig := loadYAMLConfig()
	activeConfig = selectActiveConfig(yamlConfig)
	return activeConfig
}

func selectActiveConfig(yamlConfig YamlConfig) (activeConfig ActiveConfig) {
	fmt.Printf("Selecting Active Configuration\n")

	if len(yamlConfig.ScanConfigs) == 0 {
		fmt.Printf("Missing Config Data")
		os.Exit(1)
	}

	fmt.Print("Select Config: ")
	reader := bufio.NewReader(os.Stdin)
	// ReadString will block until the delimiter is entered
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("An error occured while reading input. Please try again", err)
		return
	}

	// remove the delimeter from the string
	input = strings.TrimSuffix(input, "\n")
	fmt.Printf("Selected: %s", input)

	activeConfig.ScanConfig = yamlConfig.ScanConfigs[input]

	activeConfig.ColorMatchMap = make(map[string]ColorMatch)
	for i := 0; i < len(yamlConfig.ColorMatches); i++ {
		colorMatch := yamlConfig.ColorMatches[i]
		hash := colorMatch.hash()
		fmt.Printf("Adding Hash: %s", hash)
		activeConfig.ColorMatchMap[hash] = colorMatch
	}

	return activeConfig
}

func loadYAMLConfig() (yamlConfig YamlConfig) {
	fmt.Printf("Loading Yaml Config\n")
	yamlConfig = YamlConfig{}

	yamlFile, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}
	err = yaml.Unmarshal(yamlFile, &yamlConfig)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}
	fmt.Printf("%+v\n", yamlConfig)

	return yamlConfig
}

func waitForEnterKey() {
	fmt.Println("Press the Enter Key to Resume")
	fmt.Scanln()
	fmt.Printf("Resuming Scanning\n")
}

func timerLoop(quit <-chan bool, lock *sync.Mutex) {
	fmt.Println("Starting cycle timer noises")

	bar := progressbar.New(90)

	for {
		// sleep first, then handle the signal and/or play noise
		// time.Sleep((93600 - 1226) * 2 * time.Millisecond)
		for i := 0; i <= 90; i = i + 5 {
			bar.Add(5)
			time.Sleep(5 * time.Second)
		}

		select {
		case <-quit:
			fmt.Println("Ending cycle timer noises")
			return
		default:
			bar.Reset()
			//playChimes(lock)
		}
	}
}

func checkPixels(img *image.RGBA, activeConfig ActiveConfig) bool {
	retval := false
	for x := img.Rect.Min.X; x <= img.Rect.Max.X; x++ {
		for y := img.Rect.Min.Y; y <= img.Rect.Max.Y; y++ {
			color := img.RGBAAt(x, y)

			if val, ok := activeConfig.ColorMatchMap[strconv.Itoa(int(color.R))+strconv.Itoa(int(color.G))+strconv.Itoa(int(color.B))]; ok {
				fmt.Printf("Found: %s", val.MatchName)
				retval = true
				break
			} else {
				// Found Nothing
			}

			//if color.R == 117 && color.G == 10 && color.B == 10 {
			//	// color is red, play chime
			//	fmt.Println("Found red")
			//	retval = true
			//	break
			//} else if (color.R == 153 && color.G == 60 && color.B == 10) || (color.R == 132 && color.G == 67 && color.B == 33) || (color.R == 147 && color.G == 112 && color.B == 38) {
			//	// color is orange play chime
			//	fmt.Println("Found orange")
			//	retval = true
			//	break
			//} else if color.R == 153 && color.G == 110 && color.B == 10 {
			//	// color is yellow, play chime
			//	fmt.Println("Found yellow")
			//	retval = true
			//	break
			//} else {
			//	//fmt.Println("Found nothing")
			//}
		}
		if retval {
			break
		}
	}
	return retval
}

func captureScreen(lock *sync.Mutex, activeConfig ActiveConfig) *image.RGBA {
	n := screenshot.NumActiveDisplays()
	// fmt.Printf("Number of displays: %d\n", n)

	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)
		if bounds.Min.X == 0 && bounds.Min.Y == 0 {
			img, err := screenshot.CaptureRect(
				image.Rect(
					activeConfig.ScanConfig.BottomLeftX,
					activeConfig.ScanConfig.BottomLeftY,
					activeConfig.ScanConfig.TopRightX,
					activeConfig.ScanConfig.TopRightY))
			if err != nil {
				fmt.Printf("Failure occurred: %s\n", err)
				playCrashNoise(lock)
				panic(err)
			}

			return img
		}
	}
	return nil
}

func captureLeftScreen(activeConfig *ActiveConfig) {
	n := screenshot.NumActiveDisplays()
	fmt.Printf("Number of Active Displays: %d \n", n)
	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)
		if bounds.Min.X == 0 && bounds.Min.Y == 0 {
			img, err := screenshot.CaptureRect(
				image.Rect(
					activeConfig.ScanConfig.BottomLeftX,
					activeConfig.ScanConfig.BottomLeftY,
					activeConfig.ScanConfig.TopRightX,
					activeConfig.ScanConfig.TopRightY))
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
	//f, err := audioFiles.Open("audio/chime-sound-7143.mp3")
	f, err := audioFiles.Open("audio/evac.mp3")
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
