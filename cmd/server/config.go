package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultAddress = "127.0.0.1:19081"

type config struct {
	Address      string
	DataDir      string
	SelfCheck    bool
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func parseConfig(args []string) (config, error) {
	address := defaultAddress
	if portText := strings.TrimSpace(os.Getenv("PORT")); portText != "" {
		port, err := strconv.Atoi(portText)
		if err != nil || port < 1 || port > 65535 {
			return config{}, fmt.Errorf("PORT 必须是 1 到 65535 的端口号")
		}
		address = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	}
	flags := flag.NewFlagSet("server", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&address, "addr", address, "HTTP 监听地址")
	dataDir := flags.String("data", filepath.Join("data"), "本地持久化目录")
	selfCheck := flags.Bool("selfcheck", false, "执行真实 HTTP 全流程自检后退出")
	if err := flags.Parse(args); err != nil {
		return config{}, err
	}
	if flags.NArg() != 0 {
		return config{}, fmt.Errorf("不支持位置参数")
	}
	validated, err := validateAddress(address)
	if err != nil {
		return config{}, err
	}
	if strings.TrimSpace(*dataDir) == "" {
		return config{}, fmt.Errorf("数据目录不能为空")
	}
	return config{Address: validated, DataDir: *dataDir, SelfCheck: *selfCheck, ReadTimeout: 8 * time.Second, WriteTimeout: 15 * time.Second}, nil
}

func validateAddress(address string) (string, error) {
	host, portText, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil {
		return "", fmt.Errorf("监听地址必须采用 host:port 格式: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("监听端口必须在 1 到 65535 之间")
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return "", fmt.Errorf("监听地址必须明确使用回环主机，拒绝 %q", host)
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}
