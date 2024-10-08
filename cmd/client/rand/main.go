package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"log"
	"math/big"
	"net"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/DenzelPenzel/nyx/cmd/client/textprot"
	"github.com/DenzelPenzel/nyx/internal/common"
	"github.com/DenzelPenzel/nyx/internal/logging"
	"github.com/DenzelPenzel/nyx/internal/utils"
	"github.com/urfave/cli"
	"go.uber.org/zap"
)

var (
	taskPool = &sync.Pool{
		New: func() interface{} {
			return &Task{}
		},
	}

	metricPool = &sync.Pool{
		New: func() interface{} {
			return metric{}
		},
	}
)

type Task struct {
	Cmd   Op
	Key   []byte
	Value []byte
}

func main() {
	ctx := context.Background()
	logger := logging.WithContext(ctx)

	a := cli.NewApp()
	a.Name = "client loading tests"
	a.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "pprof-file",
			Value: "bench",
			Usage: "Create the file for the pprof data",
		},
		&cli.IntFlag{
			Name:  "num-ops",
			Value: 1000000,
			Usage: "Set up the number of ops",
		},
		&cli.IntFlag{
			Name:  "num-workers",
			Value: 10,
			Usage: "Set up the number of workers",
		},
		&cli.StringFlag{
			Name:  "hostname",
			Value: "localhost:4001",
			Usage: "HTTP server bind address",
		},
	}
	a.Action = run
	a.Commands = []cli.Command{}

	err := a.Run(os.Args)
	if err != nil {
		logger.Fatal("Error running application", zap.Error(err))
	}
}

func run(c *cli.Context) {
	pprofName := c.String("pprof-file")
	numOps := c.Int("num-ops")
	httpAddr, _ := utils.GetTCPAddr(c.String("hostname"))

	f, err := os.Create(pprofName)
	if err != nil {
		panic(err.Error())
	}
	err = pprof.StartCPUProfile(f)
	if err != nil {
		panic(err.Error())
	}

	protType := "text"
	numWorkers := runtime.GOMAXPROCS(0)
	numCmds := len(allOps)

	metrics := make(chan metric, numOps)
	tasks := make(chan *Task, numOps)

	tasksWg := &sync.WaitGroup{}
	connWg := &sync.WaitGroup{}

	opsPerTask := numOps / numCmds / numWorkers

	log.Printf("Running %v ops total with:\n"+
		"\t%v workers\n"+
		"\ttotal commands %v\n"+
		"\tusing the %v protocol\n"+
		"\toperations per task %v\n\n",
		numOps, numWorkers, allOps, protType, opsPerTask)

	for i := 0; i < numWorkers; i++ {
		tasksWg.Add(numCmds)
		for _, op := range allOps {
			go func(op Op) {
				for i := 0; i < opsPerTask; i++ {
					task := taskPool.Get().(*Task)
					task.Cmd = op
					task.Key = utils.RandData(64)
					task.Value = genData(op)
					tasks <- task
				}
				tasksWg.Done()
			}(op)
		}
	}

	for i := 0; i < numWorkers; i++ {
		connWg.Add(1)
		conn, err := utils.Connect(httpAddr)
		if err != nil {
			log.Printf("Failed connect to %s error: %s\n", httpAddr.String(), err)
			i--
			connWg.Add(-1)
			continue
		}

		go execute(conn, connWg, tasks, metrics)
	}

	stats := &sync.WaitGroup{}
	stats.Add(1)

	go func() {
		hits := make(map[Op][]int)
		misses := make(map[Op][]int)

		for m := range metrics {
			if m.miss {
				misses[m.op] = append(misses[m.op], int(m.duration))
			} else {
				hits[m.op] = append(hits[m.op], int(m.duration))
			}

			metricPool.Put(m) //nolint:staticcheck //check pointer-like issue
		}

		for i, op := range allOps {
			if i == 0 {
				log.Println("===========Metrics===========")
			}
			renderStats("hits", op, hits[op])
			renderStats("misses", op, misses[op])
			log.Println("=============================")
		}

		stats.Done()
	}()

	log.Println("Generate testing tasks...")
	tasksWg.Wait()

	log.Println("Tasks generation done")
	close(tasks)

	log.Println("Start tasks execution...")
	connWg.Wait()

	log.Println("Execution done")
	close(metrics)

	stats.Wait()
}

func execute(conn net.Conn, connWg *sync.WaitGroup, tasks <-chan *Task, metrics chan<- metric) {
	defer func() {
		conn.Close()
		connWg.Done()
	}()

	var prot textprot.TextProt
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	var err error

	for item := range tasks {
		start := time.Now()

		switch item.Cmd {
		case Set:
			err = prot.Set(rw, item.Key, item.Value)
		case Add:
			err = prot.Add(rw, item.Key, item.Value)
		case Replace:
			err = prot.Replace(rw, item.Key, item.Value)
		case Append:
			err = prot.Append(rw, item.Key, item.Value)
		case Prepend:
			err = prot.Prepend(rw, item.Key, item.Value)
		case Get:
			_, err = prot.Get(rw, item.Key)
		case Gat:
			_, err = prot.GAT(rw, item.Key)
		case Delete:
			err = prot.Delete(rw, item.Key)
		case Touch:
			err = prot.Touch(rw, item.Key)
		default:
			panic("Unhandled default case")
		}

		if err != nil {
			if !common.IsMiss(err) {
				// socket is closed
				if errors.Is(err, io.EOF) {
					log.Printf("Failed to execute request: %s, key: %s, error: %v\n", item.Cmd, item.Key, err)
					return
				}
			}
		}

		m := metricPool.Get().(metric)
		m.duration = time.Since(start).Milliseconds()
		m.op = item.Cmd
		m.miss = common.IsMiss(err)
		metrics <- m
		taskPool.Put(item)
	}
}

func genData(cmd Op) []byte {
	if cmd == Set || cmd == Add || cmd == Replace {
		x, _ := rand.Int(rand.Reader, big.NewInt(9*1024+1024))
		return utils.RandData(x.Int64())
	}
	return nil
}

func renderStats(t string, op Op, data []int) {
	if len(data) == 0 {
		log.Printf("\nNo %s %s\n", op.String(), t)
		return
	}
	s := GetStats(data)
	log.Printf("%s %s (n = %d)\n", op.String(), t, len(data))
	log.Printf("Min: %fms\n", s.Min)
	log.Printf("Max: %fms\n", s.Max)
	log.Printf("Avg: %fms\n", s.Avg)
	log.Printf("p50: %fms\n", s.P50)
	log.Printf("p75: %fms\n", s.P75)
	log.Printf("p90: %fms\n", s.P90)
	log.Printf("p95: %fms\n", s.P95)
	log.Printf("p99: %fms\n", s.P99)
}
