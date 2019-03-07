package main

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/simplechain-org/gominer/client"
	"github.com/simplechain-org/gominer/common"
	"github.com/simplechain-org/gominer/log"
	"github.com/simplechain-org/gominer/scrypt"
	"github.com/simplechain-org/gominer/utils"

	"github.com/mattn/go-colorable"
	"github.com/rcrowley/go-metrics"
	"gopkg.in/urfave/cli.v1"
)

var (
	maxUint256      = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0))
	mode       uint = 48
	gitCommit       = ""
	app             = utils.NewApp(gitCommit, "the gominer command line interface")
	minerFlags      = []cli.Flag{
		utils.StratumServer,
		utils.MinerName,
		utils.StratumPassword,
		utils.CPUs,
		utils.MinerThreads,
		utils.Verbosity,
	}

	task  atomic.Value
	job   = make(chan *Job, 100)
	meter = metrics.NewMeter()
)

func init() {
	app.Action = gominer
	app.HideVersion = true
	app.Copyright = "Copyright 2017-2018 The gominer Authors"
	app.Flags = append(app.Flags, minerFlags...)

	app.Before = func(ctx *cli.Context) error {
		glogger := log.NewGlogHandler(log.StreamHandler(colorable.NewColorableStderr(), log.TerminalFormat(true)))
		glogger.Verbosity(log.Lvl(ctx.GlobalInt(utils.Verbosity.Name)))
		log.Root().SetHandler(glogger)
		return nil
	}

	app.After = func(ctx *cli.Context) error {
		return nil
	}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func gominer(ctx *cli.Context) error {
	if len(ctx.GlobalString(utils.StratumServer.Name)) == 0 {
		log.Error("Invalid param 'server'")
		return cli.ShowAppHelp(ctx)
	}

	log.Info("miner name", "using", ctx.GlobalString(utils.MinerName.Name))

	CPUs := ctx.GlobalInt(utils.CPUs.Name)
	if CPUs < 0 {
		CPUs = 1
	}

	if CPUs > runtime.NumCPU() {
		CPUs = runtime.NumCPU()
	}

	runtime.GOMAXPROCS(CPUs)
	log.Info("CPU", "number", runtime.NumCPU(), "using", CPUs)

	c := client.NewStratumClient(ctx.GlobalString(utils.StratumServer.Name), ctx.GlobalString(utils.MinerName.Name), ctx.GlobalString(utils.StratumPassword.Name))
	serverCtx, cancel := context.WithCancel(context.Background())
	c.Start(serverCtx, cancel)
	threads := ctx.GlobalInt(utils.MinerThreads.Name)
	if threads == 0 {
		threads = runtime.NumCPU()
	}
	if threads < 0 {
		threads = 1
	}
	log.Info("Threads", "using", threads)

	found := make(chan uint64)

	_ = metrics.Register("hashRate", meter)

	for i := 0; i < threads; i++ {
		go doJob(job, found)
	}

	go func(taskChan chan *client.StratumTask) {
		var target *big.Int
		var begin, end uint64
		var hash common.Hash
		for {
			select {
			case newtask := <-taskChan:
				target = new(big.Int).Div(maxUint256, newtask.Difficulty)
				begin = newtask.NonceBegin
				end = newtask.NonceEnd
				hash = newtask.PowHash
				task.Store(newtask)

			default:
				if end > 0 && begin < end {
					job <- &Job{target: target, powHash: hash, nonce: begin}
					begin++
				}
			}
		}
	}(c.TaskChan)

	ticker := time.NewTicker(15 * time.Second)
	for {
		select {
		case <-c.Down:
			log.Warn("miner ShutDown")
			return nil
		case _ = <-ticker.C:
			log.Info("Calculating ", "hashRate", meter.RateMean())
			log.Debug("Buffered jobs", "length", len(job))
		case result := <-found:
			//task.Nonce = result
			submitTask := task.Load().(*client.StratumTask)
			c.SubmitTask(&client.StratumTask{
				submitTask.PowHash,
				submitTask.Difficulty,
				submitTask.Id,
				submitTask.NonceBegin,
				submitTask.NonceEnd,
				result,
			})
		}
	}
}

type Job struct {
	nonce   uint64
	powHash common.Hash
	target  *big.Int
}

func doJob(job <-chan *Job, found chan uint64) {
	for {
		select {
		case j, _ := <-job:
			_, result := scrypt.ScryptHash(j.powHash.Bytes(), j.nonce, mode)
			intResult := new(big.Int).SetBytes(result)
			if intResult.Cmp(j.target) <= 0 {
				go func() {
					found <- j.nonce
					log.Debug("Nonce success", "diff", j.nonce)
				}()
			}
			go func() {
				meter.Mark(1)
			}()
		}
	}
}
