package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/wcharczuk/go-chart"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

const SPLASH = `
_//        _//_////////_// _//   _/// _//////_//_//       _//_////////_///////    
_//        _//_//      _/    _//      _//    _//_/ _//   _///_//      _//    _//  
_//   _/   _//_//      _/     _//     _//    _//_// _// _ _//_//      _//    _//  
_//  _//   _//_//////  _/// _/        _//    _//_//  _//  _//_//////  _/ _//      
_// _/ _// _//_//      _/     _//     _//    _//_//   _/  _//_//      _//  _//    
_/ _/    _////_//      _/      _/     _//    _//_//       _//_//      _//    _//  
_//        _//_////////_//// _//      _//    _//_//       _//_////////_//      _//


WARNING: This tool is capable of sending very large amounts of traffic that could potentially lag or crash your system or the target. USE WITH CAUTION!
`

var target *url.URL
var saveLocation *string
var burstSize, burstCount *int
var burstDelay *time.Duration
var dotted *bool

var timings [][]time.Duration
var timingLocks []sync.Mutex

func main() {
	parseFlags()
	fmt.Printf(SPLASH)

	log.Printf("Command set with the following options:\n" +
		"Target:\t%s\n" +
		"Burst size:\t%d\n" +
		"Burst count:\t%d\n" +
		"Burst delay:\t%s\n" +
		"Save location:\t%s\n", (*target).String(), *burstSize, *burstCount, *burstDelay, *saveLocation)

	launch()

	graph, err := buildGraph()
	if err != nil {
		log.Printf("An error occurred while generating the chart: %v\n", err)
		return
	}

	fmt.Println("Webserver listening for connections on port 2500")
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		_, err := w.Write(graph.Bytes())
		fmt.Printf("%v\n", err)
	}))
	_ = http.ListenAndServe(":2500", mux)
}

func parseFlags() {
	tgt := flag.String("target", "http://127.0.0.1:80", "URL of the webserver to test")
	burstSize = flag.Int("size", 100, "the number of concurrent requests to send per burst")
	burstCount = flag.Int("count", 5, "the number of bursts to send. Higher leads to more accuracy")
	burstDelay = flag.Duration("delay", 1 * time.Second, "how long to wait between bursts. Defaults to 1s")
	saveLocation = flag.String("savefile", "./webtimer.png", "where to save the generated graph (PNG format)")
	dotted = flag.Bool("dotted", false, "whether to connect the dots on the graph")
	flag.Parse()

	if targetUrl, err := url.Parse(*tgt); err != nil {
		log.Fatalf("Error parsing target URL: %v\nExiting...\n", err)
		os.Exit(1)
	} else {
		target = targetUrl
	}

	if *burstSize < 1 {
		log.Fatalln("Burst size must be at least 1. Exiting...")
		os.Exit(1)
	}

	if *burstCount < 1 {
		log.Fatalln("Burst count must be at least 1. Exiting...")
		os.Exit(1)
	}

	if total := (*burstCount) * (*burstSize); total > 500000 {
		fmt.Printf("WARNING: your total request count is very large (%d) and may crash your computer.\n", total)
		for i := 15; i > 0; i-- {
			fmt.Printf("\rYou have *%d* seconds to cancel!", i)
			time.Sleep(1 * time.Second)
		}
		fmt.Printf("\n")
	}
}

func launch() {
	i := 0
	for range time.Tick(*burstDelay) {
		timingLocks = append(timingLocks, sync.Mutex{})
		timings = append(timings, []time.Duration{})
		sendBurst(i)

		i++
		if i >= *burstCount {
			break
		}
	}
}

func sendBurst(recordsIndex int) {
	log.Printf("Sending burst #%d...\n", recordsIndex + 1)

	c := make(chan bool)
	for i := 0; i < *burstSize; i++ {
		go sendRequest(recordsIndex, c)
	}

	failures := 0
	for i := 0; i < *burstSize; i++ {
		if <-c == false {
			failures++
		}
		fmt.Printf("\r%d requests received in burst #%d", i + 1, recordsIndex + 1)
	}
	fmt.Printf("\n")

	log.Printf("Burst #%d completed with %f%% of returns indicating failure\n\n", recordsIndex + 1, float64(failures) / float64(*burstSize) * 100)
}

var client = http.Client {
	Timeout: 2000 * time.Millisecond,
}
func sendRequest(recordIndex int, c chan bool) {
	req := &http.Request {
		Method: "GET",
		URL: target,
		Header: map[string][]string {
			"Cache-Control": {"no-cache"},
			"From": {"someuser-noreply@webtimer.null"}, // because I can, and maybe one could filter based on this server-side
		},
	}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	defer func() {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()

	timingLocks[recordIndex].Lock()
	// I chose not to defer the mutex unlock because I didn't want any delay from
	// something like the channel blocking because the receiver isn't moving fast enough, etc
	if err != nil || resp.StatusCode != 200 {
		timings[recordIndex] = append(timings[recordIndex], 0)
		timingLocks[recordIndex].Unlock()
		c<-false
	} else {
		timings[recordIndex] = append(timings[recordIndex], duration)
		timingLocks[recordIndex].Unlock()
		c<-true
	}
}

func buildGraph() (*bytes.Buffer, error) {
	fmt.Println("Building graph...")

	series := []chart.Series{}
	for i := 0; i < len(timings); i++ {
		xData, yData := []float64{}, []float64{}
		for j := 0; j < len(timings[i]); j++ {
			xData = append(xData, float64(j))
			yData = append(yData, timings[i][j].Seconds() * 1000)
		}

		s := chart.ContinuousSeries {
			XValues: xData,
			YValues: yData,
		}
		if *dotted {
			s.Style = chart.Style {
				Show:             true,
				StrokeWidth:      chart.Disabled,
				DotWidth:         1,
			}
		}
		series = append(series, s)
	}

	graph := chart.Chart {
		XAxis: chart.XAxis {
			Name: "Request Number",
			NameStyle: chart.StyleShow(),
			Style: chart.StyleShow(),
		},
		YAxis: chart.YAxis {
			Name: "RTT (ms)",
			NameStyle: chart.StyleShow(),
			Style: chart.StyleShow(),
		},
		Series: series,
	}
	buf := bytes.Buffer{}
	err := graph.Render(chart.PNG, &buf)

	return &buf, err
}