package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCommandStreamsBeforeExit 验证命令退出前可读取两种输出和静默执行状态。
func TestCommandStreamsBeforeExit(t *testing.T) {
	directory := t.TempDir()
	script := filepath.Join(directory, "command.sh")
	err := os.WriteFile(script, []byte("printf 'stdout-ready\\n'\nprintf 'stderr-ready\\n' >&2\ni=0\nwhile [ ! -f release ] && [ $i -lt 500 ]; do sleep 0.01; i=$((i+1)); done\ntest -f release\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	probe := &commandProbe{events: make(chan string, 32)}
	completed := make(chan error, 1)
	go func() {
		completed <- runProjectCommandWithOutput(directory, ".", probe, 20*time.Millisecond, "sh", script)
	}()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	seen := map[string]bool{}
	for len(seen) < 3 {
		select {
		case event := <-probe.events:
			seen[event] = true
		case err = <-completed:
			t.Fatalf("日志未在命令结束前完整出现: %v, %v", seen, err)
		case <-deadline.C:
			t.Fatalf("等待实时输出或执行状态超时: %v", seen)
		}
	}
	err = os.WriteFile(filepath.Join(directory, "release"), nil, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	err = <-completed
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"执行：sh", "工作目录：" + directory, "执行完成（耗时"} {
		if !strings.Contains(probe.String(), text) {
			t.Errorf("缺少执行上下文: %s", text)
		}
	}
}

// TestCommandPreservesFailure 验证实时输出模式保留标准错误、退出码与失败目录。
func TestCommandPreservesFailure(t *testing.T) {
	directory := t.TempDir()
	var output bytes.Buffer
	err := runProjectCommandWithOutput(directory, ".", &output, time.Second, "sh", "-c", "printf 'failure-detail\\n' >&2; exit 7")
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 7 {
		t.Fatalf("未保留命令退出码: %v", err)
	}
	if !strings.Contains(err.Error(), directory) || !strings.Contains(err.Error(), "failure-detail") {
		t.Fatalf("失败上下文不完整: %v", err)
	}
	if !strings.Contains(output.String(), "执行失败") || strings.Contains(output.String(), "执行完成") {
		t.Fatalf("命令完成状态错误: %s", output.String())
	}
}

// commandProbe 收集实时输出事件，供测试在释放子命令之前确认进度。
type commandProbe struct {
	bytes.Buffer
	events chan string
}

// Write 记录日志并通知测试已收到的标准输出、标准错误与执行状态。
func (probe *commandProbe) Write(content []byte) (int, error) {
	for _, event := range []string{"stdout-ready", "stderr-ready", "仍在执行"} {
		if strings.Contains(string(content), event) {
			select {
			case probe.events <- event:
			default:
			}
		}
	}
	return probe.Buffer.Write(content)
}
