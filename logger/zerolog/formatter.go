package zerolog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/liujitcn/kratos-kit/logger"
)

type textWriter struct {
	out                io.Writer
	timestampFieldName string
	levelFieldName     string
	messageFieldName   string
	callerFieldName    string
	colorLevel         bool
}

// newTextWriter 创建与 zap 控制台布局一致的 zerolog 文本 writer。
func newTextWriter(
	out io.Writer,
	timestampFieldName string,
	levelFieldName string,
	messageFieldName string,
	callerFieldName string,
	colorLevel bool,
) io.Writer {
	return &textWriter{
		out:                out,
		timestampFieldName: timestampFieldName,
		levelFieldName:     levelFieldName,
		messageFieldName:   messageFieldName,
		callerFieldName:    callerFieldName,
		colorLevel:         colorLevel,
	}
}

// Write 将 zerolog JSON 事件转换为统一文本格式后写入目标。
func (w *textWriter) Write(data []byte) (int, error) {
	var values map[string]any
	var decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var err = decoder.Decode(&values)
	if err != nil {
		return 0, fmt.Errorf("decode zerolog event: %w", err)
	}

	var timestamp = textValue(values[w.timestampFieldName])
	var level = strings.ToUpper(textValue(values[w.levelFieldName]))
	var caller = textValue(values[w.callerFieldName])
	var message = textValue(values[w.messageFieldName])
	delete(values, w.timestampFieldName)
	delete(values, w.levelFieldName)
	delete(values, w.callerFieldName)
	delete(values, w.messageFieldName)

	var builder strings.Builder
	if timestamp != "" {
		builder.WriteString(timestamp)
		builder.WriteByte(' ')
	}
	if w.colorLevel {
		builder.WriteString(zerologLevelColor(level))
		builder.WriteString(level)
		builder.WriteString("\x1b[0m")
	} else {
		builder.WriteString(level)
	}
	if caller != "" {
		builder.WriteByte(' ')
		builder.WriteString(caller)
	}
	if message != "" {
		builder.WriteByte(' ')
		builder.WriteString(message)
	}
	if len(values) > 0 {
		var fields string
		fields, err = logger.FormatFields(values)
		if err != nil {
			return 0, fmt.Errorf("encode zerolog fields: %w", err)
		}
		builder.WriteByte(' ')
		builder.WriteString(fields)
	}
	builder.WriteByte('\n')

	var output = builder.String()
	var written int
	written, err = io.WriteString(w.out, output)
	if err != nil {
		return 0, err
	}
	if written != len(output) {
		return 0, io.ErrShortWrite
	}
	return len(data), nil
}

// textValue 将 zerolog 标准字段转换为无引号文本。
func textValue(value any) string {
	if value == nil {
		return ""
	}
	var text, ok = value.(string)
	if ok {
		return text
	}
	return fmt.Sprint(value)
}

// zerologLevelColor 返回与 zap 控制台编码器相近的级别颜色。
func zerologLevelColor(level string) string {
	switch level {
	case "DEBUG", "TRACE":
		return "\x1b[35m"
	case "WARN", "WARNING":
		return "\x1b[33m"
	case "ERROR", "FATAL", "PANIC":
		return "\x1b[31m"
	default:
		return "\x1b[34m"
	}
}
