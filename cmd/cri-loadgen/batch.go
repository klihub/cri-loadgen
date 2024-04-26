package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"gonum.org/v1/gonum/stat"
)

type Latency = float64

type Batch struct {
	BatchName string
	Namespace string
	PodCount  int
	CtrCount  int
	Rounds    int
	Verbose   bool

	r       *runtime
	pods    []string
	ctrs    [][]string
	options []ContainerOption

	latency RuntimeLatency
	retries RetryCount
	errors  []error
}

type RuntimeLatency struct {
	RunPodSandbox    []Latency
	StopPodSandbox   []Latency
	RemovePodSandbox []Latency
	CreateContainer  []Latency
	StartContainer   []Latency
	StopContainer    []Latency
	RemoveContainer  []Latency
}

type RetryCount struct {
	RunPodSandbox int
	/*
	   StopPodSandbox   int
	   RemovePodSandbox int
	   CreateContainer  int
	   StartContainer   int
	   StopContainer    int
	   RemoveContainer  int
	*/
}

func (b *Batch) Run(r *runtime, wg *sync.WaitGroup) error {
	b.r = r
	b.errors = nil

	if b.Namespace == "" {
		b.Namespace = "test"
	}
	if b.PodCount == 0 {
		b.PodCount = 1
	}
	if b.CtrCount == 0 {
		b.CtrCount = 1
	}
	if b.Rounds == 0 {
		b.Rounds = 10
	}

	b.pods = make([]string, b.PodCount, b.PodCount)
	for i := 0; i < b.PodCount; i++ {
		for j := 0; j < b.CtrCount; j++ {
			b.ctrs = append(b.ctrs, make([]string, b.CtrCount, b.CtrCount))
		}
	}

	b.options = []ContainerOption{
		WithImage(b.r.images.pause),
		WithoutCommand(),
	}

	wg.Add(1)
	go func() {
		for i := 0; i < b.Rounds; i++ {
			fmt.Printf("batch %s, round #%d/%d\n", b.BatchName, i+1, b.Rounds)
			b.createPods()
			b.createContainers()
			b.startContainers()
			b.stopContainers()
			b.removeContainers()
			b.stopPods()
			b.removePods()
		}
		wg.Done()
	}()

	return nil
}

func (b *Batch) Errors() []error {
	return b.errors
}

const (
	maxRetries = 3
)

func (b *Batch) createPods() {
	var (
		pod string
		err error
	)

	for i := 0; i < b.PodCount; i++ {
		name := b.BatchName + "-" + strconv.Itoa(i)
		uid := ids.GenUID()
		for retry := 0; retry < maxRetries; retry++ {
			start := time.Now()
			pod, err = b.r.CreatePod(b.Namespace, name, uid)
			latency := AsLatency(time.Since(start))
			if err == nil {
				b.latency.RunPodSandbox = append(b.latency.RunPodSandbox, latency)
				b.printf("created #%d pod %s (latency %s)\n", i, pod, AsDuration(latency))
				break
			}
			b.retries.RunPodSandbox++
		}
		b.pods[i] = pod
		if err != nil {
			err = fmt.Errorf("failed to create pod #%d: %w", i, err)
			b.errors = append(b.errors, err)
			b.printf("%v\n", err)
		}
	}
}

func (b *Batch) createContainers() {
	for i, pod := range b.pods {
		for j := 0; j < b.CtrCount; j++ {
			name := "ctr-" + strconv.Itoa(j)
			uid := ids.GenUID()
			start := time.Now()
			ctr, err := b.r.CreateContainer(pod, name, uid, b.options...)
			latency := AsLatency(time.Since(start))
			b.ctrs[i][j] = ctr
			if err != nil {
				err = fmt.Errorf("failed to create container #%d:%d: %w", i, j, err)
				b.errors = append(b.errors, err)
				b.printf("%v\n", err)
			} else {
				b.latency.CreateContainer = append(b.latency.CreateContainer, latency)
				b.printf("created #%d:%d container %s (latency %s)\n", i, j, ctr, AsDuration(latency))
			}
		}
	}
}

func (b *Batch) startContainers() {
	for i, pod := range b.pods {
		if pod == "" {
			continue
		}
		for j, ctr := range b.ctrs[i] {
			if ctr == "" {
				continue
			}
			start := time.Now()
			err := b.r.StartContainer(ctr)
			latency := AsLatency(time.Since(start))
			b.ctrs[i][j] = ctr
			if err != nil {
				err = fmt.Errorf("failed to start container #%d:%d: %w", i, j, err)
				b.errors = append(b.errors, err)
				b.printf("%v\n", err)
			} else {
				b.latency.StartContainer = append(b.latency.StartContainer, latency)
				b.printf("started #%d:%d container %s (latency %s)\n", i, j, ctr, AsDuration(latency))
			}
		}
	}
}

func (b *Batch) stopContainers() {
	for i, pod := range b.pods {
		if pod == "" {
			continue
		}
		for j, ctr := range b.ctrs[i] {
			if ctr == "" {
				continue
			}
			start := time.Now()
			err := b.r.StopContainer(ctr)
			latency := AsLatency(time.Since(start))
			b.ctrs[i][j] = ctr
			if err != nil {
				err = fmt.Errorf("failed to stop container #%d:%d: %w", i, j, err)
				b.errors = append(b.errors, err)
				b.printf("%v\n", err)
			} else {
				b.latency.StopContainer = append(b.latency.StopContainer, latency)
				b.printf("stopped #%d:%d container %s (latency %s)\n", i, j, ctr, AsDuration(latency))
			}
		}
	}
}

func (b *Batch) removeContainers() {
	for i, pod := range b.pods {
		if pod == "" {
			continue
		}
		for j, ctr := range b.ctrs[i] {
			if ctr == "" {
				continue
			}
			start := time.Now()
			err := b.r.RemoveContainer(ctr)
			latency := AsLatency(time.Since(start))
			b.ctrs[i][j] = ""
			if err != nil {
				err = fmt.Errorf("failed to remove container #%d:%d: %w", i, j, err)
				b.errors = append(b.errors, err)
				b.printf("%v\n", err)
			} else {
				b.latency.RemoveContainer = append(b.latency.RemoveContainer, latency)
				b.printf("removed #%d:%d container %s (latency %s)\n", i, j, ctr, AsDuration(latency))
			}
		}
	}
}

func (b *Batch) stopPods() {
	for i, pod := range b.pods {
		if pod == "" {
			continue
		}
		start := time.Now()
		err := b.r.StopPod(pod)
		latency := AsLatency(time.Since(start))
		if err != nil {
			err = fmt.Errorf("failed to stop pod #%d: %w", i, err)
			b.errors = append(b.errors, err)
			b.printf("%v\n", err)
		} else {
			b.latency.StopPodSandbox = append(b.latency.StopPodSandbox, latency)
			b.printf("stopped #%d pod %s (latency %s)\n", i, pod, AsDuration(latency))
		}
	}
}

func (b *Batch) removePods() {
	for i, pod := range b.pods {
		if pod == "" {
			continue
		}
		start := time.Now()
		err := b.r.RemovePod(pod)
		latency := AsLatency(time.Since(start))
		b.pods[i] = ""
		if err != nil {
			err = fmt.Errorf("failed to remove pod #%d: %w", i, err)
			b.errors = append(b.errors, err)
			b.printf("%v\n", err)
		} else {
			b.latency.RemovePodSandbox = append(b.latency.RemovePodSandbox, latency)
			b.printf("removed #%d pod %s (latency %s)\n", i, pod, AsDuration(latency))
		}
	}
}

func (b *Batch) printf(format string, args ...interface{}) {
	if !b.Verbose {
		return
	}
	fmt.Printf(b.BatchName+": "+format, args...)
}

func (l *RuntimeLatency) Add(o *RuntimeLatency) {
	l.RunPodSandbox = append(l.RunPodSandbox, o.RunPodSandbox...)
	l.StopPodSandbox = append(l.StopPodSandbox, o.StopPodSandbox...)
	l.RemovePodSandbox = append(l.RemovePodSandbox, o.RemovePodSandbox...)
	l.CreateContainer = append(l.CreateContainer, o.CreateContainer...)
	l.StartContainer = append(l.StartContainer, o.StartContainer...)
	l.StopContainer = append(l.StopContainer, o.StopContainer...)
	l.RemoveContainer = append(l.RemoveContainer, o.RemoveContainer...)
}

func (l *RuntimeLatency) Sort() {
	slices.Sort(l.RunPodSandbox)
	slices.Sort(l.StopPodSandbox)
	slices.Sort(l.RemovePodSandbox)
	slices.Sort(l.CreateContainer)
	slices.Sort(l.StartContainer)
	slices.Sort(l.StopContainer)
	slices.Sort(l.RemoveContainer)
}

func (l *RuntimeLatency) Print() {

	min, max := l.RunPodSandbox[0], l.RunPodSandbox[len(l.RunPodSandbox)-1]
	mean, dev := stat.MeanStdDev(l.RunPodSandbox, nil)
	p25 := stat.Quantile(0.25, stat.Empirical, l.RunPodSandbox, nil)
	p50 := stat.Quantile(0.50, stat.Empirical, l.RunPodSandbox, nil)
	p75 := stat.Quantile(0.75, stat.Empirical, l.RunPodSandbox, nil)
	p95 := stat.Quantile(0.95, stat.Empirical, l.RunPodSandbox, nil)
	fmt.Printf("RunPodSandbox latency:\n")
	fmt.Printf("  - min, max: %s, %s, mean, deviation: %s, %s\n",
		AsDuration(min), AsDuration(max), AsDuration(mean), AsDuration(dev))
	fmt.Printf("  - {25,50,75,95}-percentiles: %s, %s, %s, %s\n",
		AsDuration(p25), AsDuration(p50), AsDuration(p75), AsDuration(p95))

	min, max = l.StopPodSandbox[0], l.StopPodSandbox[len(l.StopPodSandbox)-1]
	mean, dev = stat.MeanStdDev(l.StopPodSandbox, nil)
	p25 = stat.Quantile(0.25, stat.Empirical, l.StopPodSandbox, nil)
	p50 = stat.Quantile(0.50, stat.Empirical, l.StopPodSandbox, nil)
	p75 = stat.Quantile(0.75, stat.Empirical, l.StopPodSandbox, nil)
	p95 = stat.Quantile(0.95, stat.Empirical, l.StopPodSandbox, nil)
	fmt.Printf("StopPodSandbox latency:\n")
	fmt.Printf("  - min, max: %s, %s, mean, deviation: %s, %s\n",
		AsDuration(min), AsDuration(max), AsDuration(mean), AsDuration(dev))
	fmt.Printf("  - {25,50,75,95}-percentiles: %s, %s, %s, %s\n",
		AsDuration(p25), AsDuration(p50), AsDuration(p75), AsDuration(p95))

	min, max = l.RemovePodSandbox[0], l.RemovePodSandbox[len(l.RemovePodSandbox)-1]
	mean, dev = stat.MeanStdDev(l.RemovePodSandbox, nil)
	p25 = stat.Quantile(0.25, stat.Empirical, l.RemovePodSandbox, nil)
	p50 = stat.Quantile(0.50, stat.Empirical, l.RemovePodSandbox, nil)
	p75 = stat.Quantile(0.75, stat.Empirical, l.RemovePodSandbox, nil)
	p95 = stat.Quantile(0.95, stat.Empirical, l.RemovePodSandbox, nil)
	fmt.Printf("RemovePodSandbox latency:\n")
	fmt.Printf("  - min, max: %s, %s, mean, deviation: %s, %s\n",
		AsDuration(min), AsDuration(max), AsDuration(mean), AsDuration(dev))
	fmt.Printf("  - {25,50,75,95}-percentiles: %s, %s, %s, %s\n",
		AsDuration(p25), AsDuration(p50), AsDuration(p75), AsDuration(p95))

	min, max = l.CreateContainer[0], l.CreateContainer[len(l.CreateContainer)-1]
	mean, dev = stat.MeanStdDev(l.CreateContainer, nil)
	p25 = stat.Quantile(0.25, stat.Empirical, l.CreateContainer, nil)
	p50 = stat.Quantile(0.50, stat.Empirical, l.CreateContainer, nil)
	p75 = stat.Quantile(0.75, stat.Empirical, l.CreateContainer, nil)
	p95 = stat.Quantile(0.95, stat.Empirical, l.CreateContainer, nil)
	fmt.Printf("CreateContainer latency:\n")
	fmt.Printf("  - min, max: %s, %s, mean, deviation: %s, %s\n",
		AsDuration(min), AsDuration(max), AsDuration(mean), AsDuration(dev))
	fmt.Printf("  - {25,50,75,95}-percentiles: %s, %s, %s, %s\n",
		AsDuration(p25), AsDuration(p50), AsDuration(p75), AsDuration(p95))

	min, max = l.StartContainer[0], l.StartContainer[len(l.StartContainer)-1]
	mean, dev = stat.MeanStdDev(l.StartContainer, nil)
	p25 = stat.Quantile(0.25, stat.Empirical, l.StartContainer, nil)
	p50 = stat.Quantile(0.50, stat.Empirical, l.StartContainer, nil)
	p75 = stat.Quantile(0.75, stat.Empirical, l.StartContainer, nil)
	p95 = stat.Quantile(0.95, stat.Empirical, l.StartContainer, nil)
	fmt.Printf("StartContainer latency:\n")
	fmt.Printf("  - min, max: %s, %s, mean, deviation: %s, %s\n",
		AsDuration(min), AsDuration(max), AsDuration(mean), AsDuration(dev))
	fmt.Printf("  - {25,50,75,95}-percentiles: %s, %s, %s, %s\n",
		AsDuration(p25), AsDuration(p50), AsDuration(p75), AsDuration(p95))

	min, max = l.StopContainer[0], l.StopContainer[len(l.StopContainer)-1]
	mean, dev = stat.MeanStdDev(l.StopContainer, nil)
	p25 = stat.Quantile(0.25, stat.Empirical, l.StopContainer, nil)
	p50 = stat.Quantile(0.50, stat.Empirical, l.StopContainer, nil)
	p75 = stat.Quantile(0.75, stat.Empirical, l.StopContainer, nil)
	p95 = stat.Quantile(0.95, stat.Empirical, l.StopContainer, nil)
	fmt.Printf("StopContainer latency:\n")
	fmt.Printf("  - min, max: %s, %s, mean, deviation: %s, %s\n",
		AsDuration(min), AsDuration(max), AsDuration(mean), AsDuration(dev))
	fmt.Printf("  - {25,50,75,95}-percentiles: %s, %s, %s, %s\n",
		AsDuration(p25), AsDuration(p50), AsDuration(p75), AsDuration(p95))

	min, max = l.RemoveContainer[0], l.RemoveContainer[len(l.RemoveContainer)-1]
	mean, dev = stat.MeanStdDev(l.RemoveContainer, nil)
	p25 = stat.Quantile(0.25, stat.Empirical, l.RemoveContainer, nil)
	p50 = stat.Quantile(0.50, stat.Empirical, l.RemoveContainer, nil)
	p75 = stat.Quantile(0.75, stat.Empirical, l.RemoveContainer, nil)
	p95 = stat.Quantile(0.95, stat.Empirical, l.RemoveContainer, nil)
	fmt.Printf("RemoveContainer latency:\n")
	fmt.Printf("  - min, max: %s, %s, mean, deviation: %s, %s\n",
		AsDuration(min), AsDuration(max), AsDuration(mean), AsDuration(dev))
	fmt.Printf("  - {25,50,75,95}-percentiles: %s, %s, %s, %s\n",
		AsDuration(p25), AsDuration(p50), AsDuration(p75), AsDuration(p95))

}

func (l *RuntimeLatency) Save(fileName string) error {
	var (
		f   *os.File
		err error
	)

	if fileName == "-" {
		f = os.Stdout
	} else {
		if !strings.HasSuffix(fileName, ".json") {
			fileName += ".json"
		}
		f, err = os.Create(fileName)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", err)
		}
		defer f.Close()
	}

	enc := json.NewEncoder(f)
	err = enc.Encode(l)
	if err != nil {
		return fmt.Errorf("failed to JSON-encode results: %w", err)
	}

	return nil
}

func AsLatency(d time.Duration) float64 {
	return Latency(d.Microseconds())
}

func AsDuration(l Latency) time.Duration {
	return time.Duration(float64(time.Microsecond) * l)
}
