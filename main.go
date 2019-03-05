package main

import (
	"context"
	"fmt"
	"github.com/mattn/go-colorable"
	"github.com/rcrowley/go-metrics"
	"github.com/simplechain-org/gominer/client"
	"github.com/simplechain-org/gominer/common"
	"github.com/simplechain-org/gominer/log"
	"github.com/simplechain-org/gominer/scrypt"
	"github.com/simplechain-org/gominer/utils"
	"gopkg.in/urfave/cli.v1"
	"math/big"
	"os"
	"runtime"
	"time"
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

	task  *client.StratumTask
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

	ticker := time.NewTicker(15 * time.Second)
	for {
		select {
		case <-c.Down:
			log.Warn("miner ShutDown")
			return nil
		case _ = <-ticker.C:
			log.Info("Calculating ", "hashRate", meter.RateMean())
			log.Debug("Buffered jobs", "length", len(job))
		case task = <-c.TaskChan:
			target := new(big.Int).Div(maxUint256, task.Difficulty)
			go func(target *big.Int, begin, end uint64) {
				var i uint64
				i = begin
				diff := task.Difficulty
				for {
					// Identify task
					if begin != task.NonceBegin || diff.Cmp(task.Difficulty) != 0 {
						log.Info("go down", "nonce begin", begin, "task nonce begin", task.NonceBegin)
						break
					}
					job <- &Job{target: target, powHash: &task.PowHash, nonce: i}
					i++
					if i > end {
						break
					}
				}
			}(target, task.NonceBegin, task.NonceEnd)
		case result := <-found:
			go func() {
				task.Nonce = result
				c.SubmitTask(task)
			}()
		}
	}
}

type Job struct {
	nonce   uint64
	powHash *common.Hash
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
