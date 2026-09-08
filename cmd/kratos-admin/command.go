package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var projectProgress = log.New(os.Stdout, "", log.Ltime)

// commandOutput 串行写入子命令输出与进度，避免并发日志交错和缓冲区竞争。
type commandOutput struct {
	mu     sync.Mutex
	writer io.Writer
}

// Write 同步转发命令输出与进度信息。
func (output *commandOutput) Write(content []byte) (int, error) {
	output.mu.Lock()
	defer output.mu.Unlock()
	return output.writer.Write(content)
}

// runProjectCommand 实时执行项目命令，并保留工作目录与失败上下文。
func runProjectCommand(target, name string, args ...string) error {
	return runProjectCommandInDirectory(target, ".", name, args...)
}

// runProjectCommandInDirectory 显示当前命令、实时输出与每十秒一次的执行状态。
func runProjectCommandInDirectory(target, directory, name string, args ...string) error {
	return runProjectCommandWithOutput(target, directory, os.Stdout, 10*time.Second, name, args...)
}

// runProjectCommandWithOutput 将命令日志写入指定输出，定时报告耗时并保留失败详情。
func runProjectCommandWithOutput(target, directory string, output io.Writer, interval time.Duration, name string, args ...string) error {
	command := exec.Command(name, args...)
	command.Dir = filepath.Join(target, filepath.FromSlash(directory))
	if name == "go" {
		// 独立生成项目只使用自己的 go.mod，避免继承调用方的 Go workspace。
		command.Env = append(os.Environ(), "GOWORK=off")
	}
	var captured bytes.Buffer
	stream := &commandOutput{writer: io.MultiWriter(output, &captured)}
	command.Stdout = stream
	command.Stderr = stream
	progress := log.New(stream, "", log.Ltime)
	commandText := strings.Join(append([]string{name}, args...), " ")
	progress.Printf("执行：%s\n工作目录：%s", commandText, command.Dir)
	started := time.Now()
	err := command.Start()
	if err != nil {
		return fmt.Errorf("在 %s 启动 %s 失败: %w", command.Dir, commandText, err)
	}
	completed := make(chan error, 1)
	go func() {
		completed <- command.Wait()
	}()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case err = <-completed:
			if err != nil {
				progress.Printf("执行失败（耗时 %s）：%s", time.Since(started).Round(time.Millisecond), commandText)
				return fmt.Errorf("在 %s 执行 %s 失败: %w\n%s", command.Dir, commandText, err, strings.TrimSpace(captured.String()))
			}
			progress.Printf("执行完成（耗时 %s）：%s", time.Since(started).Round(time.Millisecond), commandText)
			return nil
		case <-ticker.C:
			progress.Printf("仍在执行（已耗时 %s）：%s", time.Since(started).Round(time.Second), commandText)
		}
	}
}
