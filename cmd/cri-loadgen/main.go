package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

func setup() *runtime {
	r, err := ConnectRuntime()
	if err != nil {
		logrus.WithError(err).Fatal("failed to connect to runtime")
	}

	err = r.PullImages()
	if err != nil {
		logrus.WithError(err).Fatal("failed to pull test images")
	}

	return r
}

func main() {
	var (
		batchCount = flag.Int("batches", 1, "number of parallel batches to run")
		podCount   = flag.Int("pods", 1, "number of pods to create per batch")
		ctrCount   = flag.Int("containers", 1, "number of containers to create per pod")
		roundCount = flag.Int("rounds", 10, "test rounds per batch")
		outFile    = flag.String("save", "", "file to save measured raw latencies in")
		verbose    = flag.Bool("verbose", false, "verbose printing during test rounds")
	)

	flag.Parse()
	r := setup()

	batches := []*Batch{}
	for i := 0; i < *batchCount; i++ {
		batches = append(batches, &Batch{
			BatchName: "batch" + strconv.Itoa(i),
			PodCount:  *podCount,
			CtrCount:  *ctrCount,
			Rounds:    *roundCount,
			Verbose:   *verbose,
		})
	}

	start := time.Now()

	wg := &sync.WaitGroup{}
	for _, b := range batches {
		b.Run(r, wg)
	}
	wg.Wait()

	elapsed := time.Since(start)

	latency := &RuntimeLatency{}
	errCnt := 0
	for _, b := range batches {
		if errs := b.Errors(); len(errs) > 0 {
			fmt.Printf("batch %s had %d errors:\n", b.BatchName, len(errs))
			for _, err := range errs {
				fmt.Printf("  %v\n", err)
			}
			errCnt += len(errs)
		}
		latency.Add(&b.latency)
	}

	fmt.Printf("%s to run %d rounds of %d batches of %d pods with %d containers\n",
		elapsed, *roundCount, *batchCount, *podCount, *ctrCount)

	if errCnt > 0 {
		fmt.Printf("encountered %d errors total\n", errCnt)
	} else {
		fmt.Printf("no errors encountered\n")
	}

	latency.Sort()
	latency.Print()
	if outFile != nil {
		if err := latency.Save(*outFile); err != nil {
			fmt.Printf("failed to save raw results in %s: %v\n", *outFile, err)
			os.Exit(1)
		}
	}
}
