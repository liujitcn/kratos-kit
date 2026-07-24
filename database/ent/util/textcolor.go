package util

// Black 返回黑色终端文本。
func Black(str string) string {
	return "\033[30m" + str + "\033[0m"
}

// Red 返回红色终端文本。
func Red(str string) string {
	return "\033[31m" + str + "\033[0m"
}

// Green 返回绿色终端文本。
func Green(str string) string {
	return "\033[32m" + str + "\033[0m"
}

// Yellow 返回黄色终端文本。
func Yellow(str string) string {
	return "\033[33m" + str + "\033[0m"
}

// Blue 返回蓝色终端文本。
func Blue(str string) string {
	return "\033[34m" + str + "\033[0m"
}

// Purple 返回紫色终端文本。
func Purple(str string) string {
	return "\033[35m" + str + "\033[0m"
}

// Cyan 返回青色终端文本。
func Cyan(str string) string {
	return "\033[36m" + str + "\033[0m"
}

// White 返回白色终端文本。
func White(str string) string {
	return "\033[37m" + str + "\033[0m"
}
