package utils

import (
	"gopkg.in/urfave/cli.v1"
	"os"
	"path/filepath"
	"runtime"
)

var (
	CommandHelpTemplate = `{{.cmd.Name}}{{if .cmd.Subcommands}}
{{if .cmd.Description}}{{.cmd.Description}}
{{end}}{{if .cmd.Subcommands}}
SUBCOMMANDS:
	{{range .cmd.Subcommands}}{{.Name}}{{with .ShortName}}, {{.}}{{end}}{{ "\t" }}{{.Usage}}
	{{end}}{{end}}{{if .categorizedFlags}}
{{range $idx, $categorized := .categorizedFlags}}{{$categorized.Name}} OPTIONS:
{{range $categorized.Flags}}{{"\t"}}{{.}}
{{end}}
{{end}}{{end}}`
	hostname, _ = os.Hostname()
)

func init() {
	cli.AppHelpTemplate = `{{.Name}} {{if .Flags}}[global options] {{end}}

VERSION:
   {{.Version}}

COMMANDS:
   {{range .Commands}}{{.Name}}{{with .ShortName}}, {{.}}{{end}}{{ "\t" }}{{.Usage}}
   {{end}}{{if .Flags}}
GLOBAL OPTIONS:
   {{range .Flags}}{{.}}
   {{end}}{{end}}
`

	cli.CommandHelpTemplate = CommandHelpTemplate
}

// NewApp creates an app with sane defaults.
func NewApp(gitCommit, usage string) *cli.App {
	app := cli.NewApp()
	app.Name = filepath.Base(os.Args[0])
	app.Author = ""
	app.Email = ""
	app.Version = "1.0"
	if len(gitCommit) >= 8 {
		app.Version += "-" + gitCommit[:8]
	}
	app.Usage = usage
	return app
}

var (
	StratumServer = cli.StringFlag{
		Name:  "server",
		Usage: "stratum server address,(host:port)",
	}

	MinerName = cli.StringFlag{
		Name:  "name",
		Usage: "miner name registered to the stratum server",
		Value: hostname,
	}

	StratumPassword = cli.StringFlag{
		Name:  "password",
		Usage: "stratum protocol password, default: no password",
	}

	Verbosity = cli.IntFlag{
		Name:  "verbosity",
		Usage: "Logging verbosity: 0=silent, 1=error, 2=warn, 3=info, 4=debug, 5=detail (default: 3)",
		Value: 3,
	}

	CPUs = cli.IntFlag{
		Name:  "cpu",
		Usage: "Sets the maximum number of CPUs that can be executing simultaneously",
		Value: runtime.NumCPU(),
	}

	MinerThreads = cli.IntFlag{
		Name:  "threads",
		Usage: "Number of CPU threads to use for mining",
		Value: runtime.NumCPU(),
	}
)
