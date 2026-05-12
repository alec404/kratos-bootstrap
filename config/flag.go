package config

import (
	"flag"
)

// CommandFlags 命令传参
type CommandFlags struct {
	Conf string // 引导配置文件路径，默认为：../../configs
}

func NewCommandFlags() *CommandFlags {
	f := &CommandFlags{
		Conf: "",
	}

	f.defineFlag()

	return f
}

func (f *CommandFlags) defineFlag() {
	flag.StringVar(&f.Conf, "conf", "../../configs", "config path, eg: -conf ../../configs")
}

func (f *CommandFlags) Init() {
	flag.Parse()
}
