// admin 是 WinCC 数据库架构管理主程序。
//
// 默认进入交互式数据库管理终端（原 main.go 全部功能，并新增管理员删表）；
// 传入 --generator 直接启动变量 C 动作生成器 Web 页面（原 action.go 功能）。
//
// 构建:
//
//	go build -o admin.exe ./cmd/admin
package main

import (
	"flag"
	"fmt"
	"os"

	"wincc2csv/internal/app"
	"wincc2csv/internal/generator"
)

func usage() {
	fmt.Println("WinCC 数据库架构管理工具")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  admin                交互式数据库管理终端 (登录/建表/删表/导出/清空)")
	fmt.Println("  admin -generator     直接启动变量 C 动作生成器 Web 页面")
	fmt.Println("  admin -h             显示帮助")
}

func main() {
	genMode := flag.Bool("generator", false, "直接启动变量 C 动作生成器 Web 页面")
	flag.Usage = func() {
		usage()
		flag.PrintDefaults()
	}
	flag.Parse()

	if *genMode {
		if err := generator.Run(); err != nil {
			fmt.Printf("❌ 生成器启动失败: %v\n", err)
			os.Exit(1)
		}
		return
	}

	manager := app.New()
	if !manager.RunManage() {
		os.Exit(0)
	}
}
