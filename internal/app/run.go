package app

import (
	"fmt"
	"strings"

	"wincc2csv/internal/generator"
)

// readOnlyExportCmd 普通模式也能导出当前表（不清理），复用 ExportOfCurrent。
func (a *App) readOnlyExportCmd() {
	if a.CurrentTable == "" {
		fmt.Println("⚠️ 尚未选择表，请先切换数据表！")
		return
	}
	if err := a.ExportOfCurrent(); err != nil {
		fmt.Printf("❌ 导出失败: %v\n", err)
	}
}

// RunManage 交互式主循环：数据库导航 + 只读视图 + 管理员架构操作。
func (a *App) RunManage() bool {
	if !a.LoginFlow() {
		return false
	}
	if !a.SelectDatabaseFlow() {
		return false
	}
	if !a.SelectTableFlow() {
		return false
	}

	for {
		a.PrintContext()

		fmt.Println("  [1] 查看当前表的所有字段结构 (列名/数据类型/约束)")
		fmt.Println("  [2] 导出当前表全量数据到 Excel/CSV (安全只读)")
		fmt.Println("  [3] 🔍 快速查看最近 20 条数据 (终端实时打印)")
		fmt.Println("  [8] 切换数据表 (Table)")
		fmt.Println("  [9] 切换数据库 (Database)")
		fmt.Println("  [S] 🧩 启动变量 C 动作生成器 Web 页面(需交互/管理员)")

		if !a.IsAdmin {
			fmt.Println("  [99] 进入高级/管理员模式 (解锁建表、字段修改与清除权限)")
		} else {
			fmt.Println("  -------------------- 管理员模式功能 --------------------")
			fmt.Println("  [10] 🚀 创建全新数据表 (支持从 Excel 批量粘贴变量列表快速生成)")
			fmt.Println("  [11] ➕ 向当前表新增字段 (支持包含 # 及中文列名)")
			fmt.Println("  [12] ➖ 从当前表删除指定字段 (ALTER TABLE DROP)")
			fmt.Println("  [13] ⚠️ 清空当前表的所有历史数据 (TRUNCATE TABLE)")
			fmt.Println("  [14] 🔥 彻底删除当前表 (DROP TABLE，结构+数据)")
			fmt.Println("  [00] 退出管理员模式")
		}

		fmt.Println("  [0] 退出程序")
		fmt.Println(strings.Repeat("=", 64))

		choice := a.Line("请输入菜单编号: ")

		switch choice {
		case "1":
			if a.CurrentTable == "" {
				fmt.Println("⚠️ 尚未选择表！")
			} else {
				a.ShowTableSchema()
			}
		case "2":
			a.readOnlyExportCmd()
		case "3":
			if a.CurrentTable == "" {
				fmt.Println("⚠️ 尚未选择表！")
			} else {
				a.PreviewRecentRecords(20)
			}
		case "8":
			a.SelectTableFlow()
		case "9":
			if a.SelectDatabaseFlow() {
				a.SelectTableFlow()
			}
		case "S", "s":
			fmt.Println("\n🖥️  正在启动 generator（Ctrl+C 或关闭窗口退出）...")
			if err := generator.Run(); err != nil {
				fmt.Printf("❌ 生成器启动失败: %v\n", err)
				a.ReadPassword("按回车返回主菜单...")
			}
		case "99":
			if !a.IsAdmin {
				a.EnterAdminRoute()
			}
		case "00":
			if a.IsAdmin {
				a.IsAdmin = false
				fmt.Println("\n🔒 已锁定权限，退回普通只读模式。")
			}
		case "10":
			if a.AdminGate() {
				a.CreateNewTable()
			}
		case "11":
			if a.AdminGate() {
				a.AddTableColumn()
			}
		case "12":
			if a.AdminGate() {
				a.DropTableColumn()
			}
		case "13":
			if a.AdminGate() {
				a.TruncateCurrentTable()
			}
		case "14":
			if a.AdminGate() {
				a.DropCurrentTable()
				if a.CurrentTable == "" && a.CurrentDB != "" {
					a.SelectTableFlow()
				}
			}
		case "0":
			fmt.Println("\n程序已安全退出。")
			return true
		default:
			fmt.Println("\n❌ 无效的输入编号或无权限执行该操作！")
		}

		a.Pause()
	}
}
