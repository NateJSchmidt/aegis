package main

import (
	"bufio"
	"embed"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
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
	QuickLoad    string                `yaml:"QuickLoad"`
	ScanConfigs  map[string]ScanConfig `yaml:"ScanConfigs"`
	ColorMatches []ColorMatch          `yaml:"ColorMatches"`
}

type ScanConfig struct {
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

type uiControls struct {
	threatScannerWidget        *widget.Check
	threatScannerLoopStatus    bool
	threatScannerRunningStatus binding.Bool

	cycleTimerWidget            *widget.Check
	cycleTimerLoopStatus        bool
	cycleTimerRunningStatus     binding.Bool
	cycleTimerProgressBarWidget *widget.ProgressBar

	myApp fyne.App
}

func main() {
	fmt.Println("This program runs indefintely, ctrl+c to exit.")

	activeConfig := setup()

	ui := configureGUILayout()

	saveScreenCapture(activeConfig)

	cycleTimerQuitChannel := make(chan bool, 5)
	threatScannerQuitChannel := make(chan bool, 5)
	var lock sync.Mutex

	ui.cycleTimerRunningStatus.AddListener(binding.NewDataListener(func() {
		drainChannel(cycleTimerQuitChannel)
		value, err := ui.cycleTimerRunningStatus.Get()
		if err != nil {
			fmt.Printf("Error fetching the value from cycle timer status: %s\n", err)
			playCrashNoise(&lock)
			panic(err)
		} else {
			if value {
				// if the check box was clicked to true, then start the go routine if not already active
				if !ui.cycleTimerLoopStatus {
					go timerLoop(cycleTimerQuitChannel, &lock, ui)
				}

				// if the loop is already active, then there is nothing to do

			} else {
				// if the check box was clicked to false, then stop the go routine
				cycleTimerQuitChannel <- true
			}
		}
	}))

	ui.threatScannerRunningStatus.AddListener(binding.NewDataListener(func() {
		drainChannel(threatScannerQuitChannel)

		value, err := ui.threatScannerRunningStatus.Get()
		fmt.Printf("threatScannerRunningStatus called with value of %v\n", value)
		if err != nil {
			fmt.Printf("Error fetching the value from threat scanner runner status: %s\n", err)
			playCrashNoise(&lock)
			panic(err)
		} else {
			if value {
				// if the check box was clicked to true, then start the go routine if not already active
				if !ui.threatScannerLoopStatus {
					go threatScanLoop(threatScannerQuitChannel, &lock, ui, activeConfig)
				}

				// if the loop is already active, then there is nothing to do

			} else {
				// if the check box was clicked to false, then stop the go routine
				threatScannerQuitChannel <- true
			}
		}
	}))

	ui.myApp.Run()
	threatScannerQuitChannel <- true
	cycleTimerQuitChannel <- true
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

	if len(yamlConfig.QuickLoad) > 0 {
		fmt.Printf("Quick Loading Profile: %s\n", yamlConfig.QuickLoad)
		activeConfig.ScanConfig = yamlConfig.ScanConfigs[yamlConfig.QuickLoad]
	} else {
		fmt.Print("Select Config: ")
		reader := bufio.NewReader(os.Stdin)
		// ReadString will block until the delimiter is entered
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("An error occured while reading input. Please try again", err)
			return
		}

		// remove the delimeter from the string
		input = strings.TrimSpace(input)
		fmt.Printf("Selected: %s\n", input)
		activeConfig.ScanConfig = yamlConfig.ScanConfigs[input]
	}

	activeConfig.ColorMatchMap = make(map[string]ColorMatch)
	for i := 0; i < len(yamlConfig.ColorMatches); i++ {
		colorMatch := yamlConfig.ColorMatches[i]
		hash := colorMatch.hash()
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

func threatScanLoop(quit <-chan bool, lock *sync.Mutex, ui *uiControls, activeConfig ActiveConfig) {
	fmt.Println("Starting threat scanner")
	ui.threatScannerLoopStatus = true

	for {
		select {
		case <-quit:
			fmt.Println("Threat scanner flipping off.")
			ui.threatScannerLoopStatus = false
			return
		default:
			img := captureScreen(lock, activeConfig)

			foundBaddie := checkPixels(img, activeConfig)

			if foundBaddie {
				playChime(lock)

				// this eventually kills this loop via the quit channel
				ui.threatScannerRunningStatus.Set(false)
				time.Sleep(1 * time.Second)
			} else {
				time.Sleep(1 * time.Second)
			}
		}
	}
}

func updateCycleTimeProgressBar(ui *uiControls, totalCycleTimeInDuration time.Duration) {
	totalTime := int64(totalCycleTimeInDuration)
	for i := int64(0); i < totalTime; i += int64(250 * time.Millisecond) {
		ui.cycleTimerProgressBarWidget.SetValue(float64(i) / float64(totalTime))
		time.Sleep(250 * time.Millisecond)
		// fmt.Printf("Updating the progress bar with %v\n", float64(i)/float64(totalCycleTimeInMs))
	}
	ui.cycleTimerProgressBarWidget.SetValue(1.0)
}

func timerLoop(quit <-chan bool, lock *sync.Mutex, ui *uiControls) {
	fmt.Println("Starting cycle timer noises")
	ui.cycleTimerLoopStatus = true

	// cycleTimeInDuration := (93200 - 1226) * 2 * time.Millisecond

	//mercoxit in hulk
	// cycleTimeInDuration := (87600 - 1226) * 2 * time.Millisecond

	// cycleTimeInDuration := (42600 - 1226) * time.Millisecond
	cycleTimeInDuration := 5 * time.Minute
	// cycleTimeInDuration := 3 * time.Second

	for {

		go updateCycleTimeProgressBar(ui, cycleTimeInDuration)

		// sleep first, then handle the signal and/or play noise
		time.Sleep(cycleTimeInDuration)

		select {
		case <-quit:
			fmt.Println("Ending cycle timer noises")
			ui.cycleTimerLoopStatus = false
			return
		default:
			playChimes(lock)
		}
	}
}

func drainChannel(ch <-chan bool) {
	fmt.Println("Starting to drain channel")
	isNotEmpty := true
	for isNotEmpty {
		select {
		case <-ch:
			fmt.Println("\tFound a signal, clearing it...")
		default:
			fmt.Println("Channel drained")
			isNotEmpty = false
		}
	}
}

func checkPixels(img *image.RGBA, activeConfig ActiveConfig) bool {
	retval := false
	for x := img.Rect.Min.X; x <= img.Rect.Max.X; x++ {
		for y := img.Rect.Min.Y; y <= img.Rect.Max.Y; y++ {
			color := img.RGBAAt(x, y)

			// if val, ok := activeConfig.ColorMatchMap[strconv.Itoa(int(color.R))+strconv.Itoa(int(color.G))+strconv.Itoa(int(color.B))]; ok {
			// 	fmt.Printf("Found: %s\n", val.MatchName)

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
			}
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

	captureRegion := image.Rect(
		activeConfig.ScanConfig.BottomLeftX,
		activeConfig.ScanConfig.BottomLeftY,
		activeConfig.ScanConfig.TopRightX,
		activeConfig.ScanConfig.TopRightY)

	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)
		if captureRegion.In(bounds) {
			img, err := screenshot.CaptureRect(captureRegion)
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

func saveScreenCapture(activeConfig ActiveConfig) {
	n := screenshot.NumActiveDisplays()
	// fmt.Printf("Number of displays: %d\n", n)

	captureRegion := image.Rect(
		activeConfig.ScanConfig.BottomLeftX,
		activeConfig.ScanConfig.BottomLeftY,
		activeConfig.ScanConfig.TopRightX,
		activeConfig.ScanConfig.TopRightY)

	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)
		if captureRegion.In(bounds) {
			img, err := screenshot.CaptureRect(captureRegion)
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

func configureGUILayout() *uiControls {
	fyneApp := app.New()
	fyneWindow := fyneApp.NewWindow("Aegis")

	retval := &uiControls{
		myApp:                       fyneApp,
		threatScannerRunningStatus:  binding.NewBool(),
		threatScannerLoopStatus:     false,
		cycleTimerRunningStatus:     binding.NewBool(),
		cycleTimerLoopStatus:        false,
		cycleTimerProgressBarWidget: widget.NewProgressBar(),
	}

	// setup the controls
	threatScannerWidget := widget.NewCheckWithData("", retval.threatScannerRunningStatus)
	cycleTimerWidget := widget.NewCheckWithData("", retval.cycleTimerRunningStatus)

	retval.threatScannerWidget = threatScannerWidget
	retval.cycleTimerWidget = cycleTimerWidget

	// setup the labels
	hLayout := container.NewHBox(
		container.NewGridWithRows(3,
			widget.NewLabel("Threat Scanner"),
			widget.NewLabel("Cycle Timer"),
			// widget.NewLabel("Place holder"),
		),
		container.NewGridWithRows(3,
			threatScannerWidget,
			cycleTimerWidget,
			// widget.NewSlider(0, 1),
		),
		container.NewGridWithRows(3,
			layout.NewSpacer(),
			retval.cycleTimerProgressBarWidget,
		),
	)
	fyneWindow.SetContent(hLayout)

	fyneWindow.Show()
	fyneWindow.SetMaster()

	return retval
}
