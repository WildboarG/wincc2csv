// scriptgen 是独立的 WinCC 变量 C 动作脚本生成器（等价原 action.go）。
//
// 与 admin 内置的 generator 功能相同，但可单独构建为轻量 exe，
// 适合只使用生成器、不需要数据库管理终端的场景。
//
// 构建:
//
//	go build -o scriptgen.exe ./cmd/scriptgen
package main

import (
	"fmt"
	"os"

	"wincc2csv/internal/generator"
)

func main() {
	if err := generator.Run(); err != nil {
		fmt.Printf("❌ 生成器启动失败: %v\n", err)
		os.Exit(1)
	}
}
