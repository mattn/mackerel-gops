package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var (
	names = map[string]string{
		"alloc":         "bytes",
		"sys":           "bytes",
		"lookups":       "integer",
		"mallocs":       "integer",
		"frees":         "integer",
		"heap_alloc":    "bytes",
		"heap_sys":      "bytes",
		"heap_idle":     "bytes",
		"heap_in_use":   "bytes",
		"heap_released": "bytes",
		"heap_objects":  "integer",
		"stack_in_use":  "bytes",
		"stack_sys":     "bytes",
	}
	proc    = flag.Int("p", 0, "process id")
	prefix  = flag.String("name", "", "prefix of keys for metrics")
	service = flag.String("service", "", "service name")
	sleep   = flag.Duration("sleep", 5*time.Second, "sleep")
)

type metric struct {
	Name  string  `json:"name"`
	Time  int64   `json:"time"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

func main() {
	flag.Parse()

	if *prefix == "" || *service == "" || *proc == 0 {
		flag.Usage()
		os.Exit(2)
	}

	if os.Getenv("MACKEREL_API_KEY") == "" {
		log.Fatal("require $MACKEREL_API_KEY")
	}

	os.Setenv("GODEBUG", "http2client=0")

	for {
		time.Sleep(*sleep)

		b, err := exec.Command("gops", "memstats", "-p", fmt.Sprint(*proc)).CombinedOutput()
		if err != nil {
			log.Fatal(err)
		}
		var metrics []metric
		for _, line := range strings.Split(string(b), "\n") {
			tokens := strings.SplitN(line, ":", 2)
			if len(tokens) != 2 {
				continue
			}
			name := strings.Replace(tokens[0], "-", "_", -1)
			unit, ok := names[name]
			if !ok {
				continue
			}
			val, _ := strconv.ParseFloat(strings.Split(strings.TrimSpace(tokens[1]), " ")[0], 64)
			metrics = append(metrics, metric{Name: *prefix + "." + name, Time: time.Now().Unix(), Value: val, Unit: unit})
		}

		var buf bytes.Buffer
		err = json.NewEncoder(&buf).Encode(metrics)
		if err != nil {
			log.Print(err)
			continue
		}
		log.Print(buf.String())
		req, err := http.NewRequest("POST", "https://mackerel.io/api/v0/services/"+*service+"/tsdb", &buf)
		if err != nil {
			log.Print(err)
			continue
		}
		req.Header.Set("X-Api-Key", os.Getenv("MACKEREL_API_KEY"))
		req.Header.Set("content-type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Print(err)
		}
		resp.Body.Close()
	}
}
