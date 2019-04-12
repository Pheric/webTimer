package main

import (
	"bytes"
	"fmt"
	"github.com/wcharczuk/go-chart"
	"log"
	"net/http"
	"net/url"
	"os"
)

const PORT = 2500
const COUNT = 100

func main() {
	u, err := url.Parse("http://localhost:2600")
	if err != nil {
		log.Fatalf("Error parsing URL: %v\n", err)
		os.Exit(1)
	}

	c := make(chan bool)
	go func(c chan bool) {
		for completed := 0; completed < COUNT; completed++ {
			<-c
			fmt.Printf("\r%d requests completed", completed + 1)
		}

		hostGraph()
	}(c)

	fmt.Println("Beginning request flood...")
	for i := 0; i < COUNT; i++ {
		go MakeRequest(u, c)
	}

	select {} // block
}
func hostGraph() {
	println("\nDrawing graph...")
	buf := bytes.Buffer{}
	cX, cY := buildChartXY()
	mainSeries := chart.ContinuousSeries {
		Style: chart.Style{
			Show:             true,
			StrokeWidth:      chart.Disabled,
			DotWidth:         1,
		},
		XValues: cX,
		YValues: cY,
	}

	graph := chart.Chart{
		XAxis: chart.XAxis{
			Name:      "Request Number",
			NameStyle: chart.StyleShow(),
			Style:     chart.StyleShow(),
		},
		YAxis: chart.YAxis{
			Name:      "Response Delay (ms)",
			NameStyle: chart.StyleShow(),
			Style:     chart.StyleShow(),
		},
		Series: []chart.Series{
			mainSeries,
		},
	}
	graph.Render(chart.PNG, &buf)

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		_, err := w.Write(buf.Bytes())
		fmt.Printf("%v\n", err)
	}))
	FreeRequests()
	fmt.Printf("%d successful responses (loss of %f%%)\n", len(cX), 100 - float32(len(cX)) / COUNT * 100)

	fmt.Printf("Listening for connections on port %d\n", PORT)
	http.ListenAndServe(fmt.Sprintf(":%d", PORT), mux)
}

func buildChartXY() ([]float64, []float64) {
	var x []float64
	var y []float64

	requests := GetRequests()
	ct := 0
	for i, r := range requests {
		if r.ResponseCode != 200 {
			i--
			continue
		}

		x = append(x, float64(i + 1))
		y = append(y, r.RequestDuration.Seconds() * 100)
		ct++
	}
	fmt.Println(ct)

	return x, y
}