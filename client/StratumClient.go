package client

import (
	"bufio"
	"context"
	"encoding/json"
	"math/big"
	"math/rand"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/simplechain-org/gominer/common"
	"github.com/simplechain-org/gominer/common/hexutil"
	"github.com/simplechain-org/gominer/log"
)

type StratumResponse struct {
	Error  interface{}   `json:"error"`
	Id     interface{}   `json:"id"`
	Result interface{}   `json:"result"`
	Params []interface{} `json:"params"`
	Method string        `json:"method"`
}

type StratumTask struct {
	PowHash    common.Hash
	Difficulty *big.Int
	Id         interface{}
	NonceBegin uint64
	NonceEnd   uint64
	Nonce      uint64
}

//{
//"id": 244,
//"method": "mining.submit",
//"params": [
//"miner1",
//"bf0488aa", // job ID, 16进制, 需要与矿池发布的 job ID 对应
//"6a909d9bbc0fa62d" // nNonce
//]
//}\n

type StratumRequest struct {
	Id     interface{}
	Method interface{}
	Params []interface{}
}

func (this *StratumRequest) Json() []byte {
	this.Id = hexutil.EncodeUint64(uint64(rand.Int31()))[2:]
	r, _ := json.Marshal(this)
	r = append(r, []byte("\n")...)
	return r
}

type StratumClient struct {
	reader        *bufio.Reader
	conn          net.Conn
	address       string
	TaskChan      chan *StratumTask
	recentTaskId  atomic.Value
	receivedNonce chan *StratumTask
	MinerName     string
	minerPasswd   string
	closed        int64
	Down          chan bool
}

func NewStratumClient(address string, minerName string, passwd string) *StratumClient {
	c := &StratumClient{
		MinerName:     minerName,
		minerPasswd:   passwd,
		address:       address,
		TaskChan:      make(chan *StratumTask, 100),
		receivedNonce: make(chan *StratumTask, 100),
		Down:          make(chan bool, 1),
	}
	return c
}

func (this *StratumClient) Connetion() {
	log.Info("Client start connection")
	for {
		var err error
		if this.conn, err = net.Dial("tcp", this.address); err == nil {
			//deadline := time.Now().Add(30 * time.Second)
			//this.conn.SetDeadline(deadline)
			this.reader = bufio.NewReaderSize(this.conn, 1000)
			subscribe := StratumRequest{Params: []interface{}{this.MinerName + "/pool.1.0.0", "Pool/1.0.0"}, Method: "mining.subscribe"}
			log.Debug("Client send", "Msg", string(subscribe.Json()))
			if _, err := this.conn.Write(subscribe.Json()); err != nil {
				log.Warn("Client send", "err", err.Error())
			} else {
				auth := StratumRequest{Params: []interface{}{this.MinerName, this.minerPasswd}, Method: "mining.authorize"}
				log.Debug("Client send", "Msg", string(auth.Json()))
				if _, err := this.conn.Write(auth.Json()); err != nil {
					log.Warn("Client send", "err", err.Error())
				} else {
					break
				}
			}
		} else {
			log.Warn("Client connection", "err", err.Error())
			time.Sleep(3 * time.Second)
		}
		time.Sleep(3 * time.Second)
	}
	log.Info("Client connected")
}

func (this *StratumClient) Start(ctx context.Context, cancel context.CancelFunc) {
	this.Connetion()
	go this.ReceiveMsg(ctx, cancel)
	go this.SendNonce(ctx, cancel)
}

func (this *StratumClient) ReceiveMsg(ctx context.Context, cancel context.CancelFunc) {
	defer func() {
		if err := recover(); err != nil {
			log.Info("ReceiveMsg", "ReceiveMsg err", err)
		}
		this.Close(cancel)
	}()
	for {
		select {
		case <-ctx.Done():
			log.Info("ReceiveMsg closed")
			return
		default:
			if line, isPrefix, err := this.reader.ReadLine(); err != nil {
				this.Connetion()
			} else if isPrefix {
				this.conn.Close()
				time.Sleep(1 * time.Second)
				this.Connetion()
			} else {
				var resp StratumResponse
				log.Debug("ReceiveMsg", "receive", string(line))
				if err := json.Unmarshal(line, &resp); err == nil {
					switch resp.Method {
					case "mining.notify":
						{
							if len(resp.Params) >= 7 {
								taskid, nonceBegin, nonceEnd, hash, diff, ok := GetTask(&resp)
								if ok {
									log.Info("ReceivedTask", "id", taskid, "diff", diff, "nonceBegin", nonceBegin, "nonceEnd", nonceEnd, "qj", nonceEnd-nonceBegin)
									this.recentTaskId.Store(taskid)

									this.TaskChan <- &StratumTask{
										PowHash:    hash,
										Difficulty: diff,
										Id:         taskid,
										NonceBegin: nonceBegin,
										NonceEnd:   nonceEnd,
									}
								}
							}

						}
					case "mining.auth_error":
						{
							log.Warn("Auth_error", "Msg:", resp.Params[0].(string))
							this.Close(cancel)
						}
					default:
						{
							log.Debug("ReceiveMsg", "Msg:", string(line))
						}
					}
				} else {
					log.Info("ReceiveMsg", "Msg", err.Error())
					this.Close(cancel)
				}
			}

		}

	}
}

func (this *StratumClient) SubmitTask(task *StratumTask) {
	if this.recentTaskId.Load().(interface{}) != nil {
		this.receivedNonce <- task
	}
}

func (this *StratumClient) SendNonce(ctx context.Context, cancel context.CancelFunc) {
	defer func() {
		if err := recover(); err != nil {
			log.Info("SendNonce", "SendNonce err", err)
		}
		this.Close(cancel)
	}()
	for {
		select {
		case <-ctx.Done():
			log.Info("SendNonce closed")
			return
		case task := <-this.receivedNonce:
			{
				if task.Id == this.recentTaskId.Load().(interface{}) && task.Nonce != 0 && this.conn != nil {
					var resp StratumRequest
					resp.Method = "mining.submit"
					resp.Params = append(resp.Params, this.MinerName)
					resp.Params = append(resp.Params, task.Id)
					resp.Params = append(resp.Params, task.Id)
					nonceHex := hexutil.EncodeUint64(task.Nonce)[2:]
					resp.Params = append(resp.Params, nonceHex)
					resp.Params = append(resp.Params, nonceHex)
					if _, err := this.conn.Write(resp.Json()); err != nil {
						log.Info("SendNonce", "err", err.Error())
					} else {
						log.Info("SendNonce success", "nonce", task.Nonce, "diff", task.Difficulty)
					}
				}
			}
		}
	}
}

func GetTask(resp *StratumResponse) (id interface{}, nonceBegin, nonceEnd uint64, hash common.Hash, diff *big.Int, ok bool) {
	var err error
	id = resp.Params[0]
	begin := resp.Params[2].(string)
	end := resp.Params[3].(string)
	hashStr := resp.Params[1].(string)
	diffStr := resp.Params[4].(string)
	if len(hashStr) < 64 {
		return
	}
	if nonceBegin, err = strconv.ParseUint(begin, 16, 64); err != nil {
		log.Warn("GetTask", "error", err.Error())
		return
	}

	if nonceEnd, err = strconv.ParseUint(end, 16, 64); err != nil {
		log.Warn("GetTask", "error", err.Error())
		return
	}

	if h, err := hexutil.Decode("0x" + hashStr[0:64]); err != nil {
		log.Warn("GetTask", "error", err.Error())
		return
	} else {
		hash = common.BytesToHash(h) //截断一半
	}
	if d, err := hexutil.DecodeBig("0x" + diffStr); err == nil {
		diff = d
	} else {
		return
	}
	log.Debug("GetTask", "difficulty", diff.String())
	if diff.Cmp(big.NewInt(0)) > 0 {
		ok = true
		return
	}
	return
}

func (this *StratumClient) Close(cancel context.CancelFunc) {
	//delete once
	if atomic.CompareAndSwapInt64(&this.closed, 0, 1) {
		log.Info("Connection closed, client shutdown.", "MinerName", this.MinerName)
		cancel()
		this.conn.Close()
		this.Down <- true
	}
}
