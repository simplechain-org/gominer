#gominer
CPU miner for simpleChain in go

## Binary releases

## Installation from source

### Prerequisites
* go version 1.10.1 or above (earlier version might work or not), check with `go version`
* gcc

```
go get github.com/simplechain-org/gominer
```

## Run
```
gominer
```

Usage:
```
  gominer [global options] 
  
  VERSION:
     1.0
  
  COMMANDS:
     help  Shows a list of commands or help for one command
     
  GLOBAL OPTIONS:
     --server value     stratum server address,(host:port)
     --name value       miner name registered to the stratum server (default: "qkl.lan")
     --password value   stratum protocol password, default: no password
     --cpu value        Sets the maximum number of CPUs that can be executing simultaneously (default: 8)
     --threads value    Number of CPU threads to use for mining (default: 8)
     --verbosity value  Logging verbosity: 0=silent, 1=error, 2=warn, 3=info, 4=debug, 5=detail (default: 3) (default: 3)
     --help, -h         show help
```

##EXAMPLES
```
./gominer --server host:port --name x --password  x --threads 2
```