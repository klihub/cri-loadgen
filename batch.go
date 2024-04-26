package main

import (
	"fmt"
	"strconv"
	"sync"

	_ "github.com/sirupsen/logrus"
)

type Batch struct {
	BatchName string
	Namespace string
	PodCount  int
	CtrCount  int
	Rounds    int

	r       *runtime
	errors  []error
	pods    []string
	ctrs    [][]string
	options []ContainerOption
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
			fmt.Printf("batch %s, round #%d/%d\n", b.BatchName, i, b.Rounds)

			b.createPods()
			b.createContainers()
			b.startContainers()
			b.stopContainers()
			b.removeContainers()
			b.removePods()
		}

		wg.Done()
	}()

	return nil
}

func (b *Batch) createPods() {
	for i := 0; i < b.PodCount; i++ {
		name := b.BatchName + "-" + strconv.Itoa(i)
		uid := ids.GenUID()
		pod, err := b.r.CreatePod(b.Namespace, name, uid)
		b.pods[i] = pod
		if err != nil {
			err = fmt.Errorf("failed to create pod #%d: %w", i, err)
			b.errors = append(b.errors, err)
			fmt.Printf("%v\n", err)
		} else {
			fmt.Printf("created #%d pod %s\n", i, pod)
		}
	}
}

func (b *Batch) createContainers() {
	for i, pod := range b.pods {
		for j := 0; j < b.CtrCount; j++ {
			name := "ctr-" + strconv.Itoa(j)
			uid := ids.GenUID()
			ctr, err := b.r.CreateContainer(pod, name, uid, b.options...)
			b.ctrs[i][j] = ctr
			if err != nil {
				err = fmt.Errorf("failed to create container #%d:%d: %w", i, j, err)
				b.errors = append(b.errors, err)
				fmt.Printf("%v\n", err)
			} else {
				fmt.Printf("created #%d:%d container %s\n", i, j, ctr)
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
			err := b.r.StartContainer(ctr)
			b.ctrs[i][j] = ctr
			if err != nil {
				err = fmt.Errorf("failed to start container #%d:%d: %w", i, j, err)
				b.errors = append(b.errors, err)
				fmt.Printf("%v\n", err)
			} else {
				fmt.Printf("started #%d:%d container %s\n", i, j, ctr)
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
			err := b.r.StopContainer(ctr)
			b.ctrs[i][j] = ctr
			if err != nil {
				err = fmt.Errorf("failed to stop container #%d:%d: %w", i, j, err)
				b.errors = append(b.errors, err)
				fmt.Printf("%v\n", err)
			} else {
				fmt.Printf("stopped #%d:%d container %s\n", i, j, ctr)
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
			err := b.r.RemoveContainer(ctr)
			b.ctrs[i][j] = ""
			if err != nil {
				err = fmt.Errorf("failed to remove container #%d:%d: %w", i, j, err)
				b.errors = append(b.errors, err)
				fmt.Printf("%v\n", err)
			} else {
				fmt.Printf("removed #%d:%d container %s\n", i, j, ctr)
			}
		}
	}
}

func (b *Batch) stopPods() {
	for i, pod := range b.pods {
		if pod == "" {
			continue
		}
		err := b.r.StopPod(pod)
		b.pods[i] = ""
		if err != nil {
			err = fmt.Errorf("failed to stop pod #%d: %w", i, err)
			b.errors = append(b.errors, err)
			fmt.Printf("%v\n", err)
		} else {
			fmt.Printf("stopped #%d pod %s\n", i, pod)
		}
	}
}

func (b *Batch) removePods() {
	for i, pod := range b.pods {
		if pod == "" {
			continue
		}
		err := b.r.RemovePod(pod)
		b.pods[i] = ""
		if err != nil {
			err = fmt.Errorf("failed to remove pod #%d: %w", i, err)
			b.errors = append(b.errors, err)
			fmt.Printf("%v\n", err)
		} else {
			fmt.Printf("removed #%d pod %s\n", i, pod)
		}
	}
}
