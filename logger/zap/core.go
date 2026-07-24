package zap

import "go.uber.org/zap/zapcore"

type ansiCleanCore struct {
	zapcore.Core
}

// newANSICleanCore 创建写入前清理 ANSI 控制码的 zap core。
func newANSICleanCore(core zapcore.Core) zapcore.Core {
	return &ansiCleanCore{Core: core}
}

// With 复制 core 并清理固定字段中的 ANSI 控制码。
func (c *ansiCleanCore) With(fields []zapcore.Field) zapcore.Core {
	return &ansiCleanCore{Core: c.Core.With(cleanANSIFields(fields))}
}

// Write 写入日志前清理消息和字符串字段中的 ANSI 控制码。
func (c *ansiCleanCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	entry.Message = stripANSI(entry.Message)
	return c.Core.Write(entry, cleanANSIFields(fields))
}

// cleanANSIFields 清理字符串字段中的 ANSI 控制码。
func cleanANSIFields(fields []zapcore.Field) []zapcore.Field {
	if len(fields) == 0 {
		return fields
	}

	cleanedFields := make([]zapcore.Field, len(fields))
	copy(cleanedFields, fields)
	for i := range cleanedFields {
		if cleanedFields[i].Type == zapcore.StringType {
			cleanedFields[i].String = stripANSI(cleanedFields[i].String)
		}
	}
	return cleanedFields
}
