package bootstrap

import (
	"github.com/spf13/cobra"
)

var (
	flags = NewCommandFlags()
)

// CommandFlags 命令传参
type CommandFlags struct {
	Conf       string // 引导配置文件路径，默认为：../../configs
	Env        string // 开发环境：dev、debug……
	ConfigHost string // 远程配置服务端地址
	ConfigType string // 远程配置服务端类型
	Daemon     bool   // 是否转为守护进程
	Project    string // 项目标识
	AppID      string // 应用标识
	InstanceID string // 实例标识
	Name       string // 应用名称
	Version    string // 应用版本
}

// NewCommandFlags 创建默认命令行参数。
func NewCommandFlags() *CommandFlags {
	return &CommandFlags{
		Conf:       "configs",
		Env:        "dev",
		ConfigHost: "127.0.0.1:8500",
		ConfigType: "consul",
		Daemon:     false,
	}
}

// AddFlags 将 flags 绑定到传入的 cobra.Command（通常是 root command）。
func (f *CommandFlags) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&f.Conf, "conf", "c", f.Conf, "config path, eg: -c configs")
	cmd.PersistentFlags().StringVarP(&f.Env, "env", "e", f.Env, "runtime environment, eg: -e dev")
	cmd.PersistentFlags().StringVarP(&f.ConfigHost, "chost", "s", f.ConfigHost, "config server host, eg: -chost 127.0.0.1:8500")
	cmd.PersistentFlags().StringVarP(&f.ConfigType, "ctype", "t", f.ConfigType, "config server type, eg: -ctype consul")
	cmd.PersistentFlags().BoolVarP(&f.Daemon, "daemon", "d", f.Daemon, "run app as a daemon with -d or --daemon")
	cmd.PersistentFlags().StringVarP(&f.Project, "project", "p", f.Project, "application project")
	cmd.PersistentFlags().StringVarP(&f.AppID, "app-id", "a", f.AppID, "application id")
	cmd.PersistentFlags().StringVarP(&f.InstanceID, "instance-id", "i", f.InstanceID, "application instance id")
	cmd.PersistentFlags().StringVarP(&f.Name, "name", "n", f.Name, "application name")
	cmd.PersistentFlags().StringVarP(&f.Version, "version", "v", f.Version, "application version")
}

func (f *CommandFlags) Init() {
	if f.Daemon {
		BeDaemon("-d")
	}
}

// NewRootCmd 创建根命令并绑定命令行参数和执行函数。
func NewRootCmd(f *CommandFlags, runE func(cmd *cobra.Command, args []string) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "A microservice server application",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			f.Init()
		},
		RunE: runE,
	}
	f.AddFlags(cmd)
	return cmd
}
