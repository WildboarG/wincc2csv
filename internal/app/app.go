package app

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/term"

	"wincc2csv/internal/exporter"
	"wincc2csv/internal/sqlserver"
)

// 管理员口令。
const AdminPassword = "7455608"

// App 承载交互式 SQL Server 架构管理主循环。
type App struct {
	Reader *bufio.Reader

	Server string
	Port   int
	User   string
	Pwd    string

	CurrentDB    string
	CurrentTable string
	IsAdmin      bool
}

// New 构造 App 实例。
func New() *App {
	return &App{
		Reader: bufio.NewReader(os.Stdin),
		Server: "127.0.0.1",
		Port:   1433,
		User:   "sa",
		Pwd:    "",
	}
}

// SQLConf 生成面向 sqlserver.Config 的配置。
func (a *App) SQLConf(dbName string) sqlserver.Config {
	return sqlserver.Config{
		Server:   a.Server,
		Port:     a.Port,
		User:     a.User,
		Password: a.Pwd,
		Database: dbName,
		Timeout:  5,
	}
}

// SQLConnect 打开（不 ping）目标库连接。
func (a *App) SQLConnect(dbName string) (*sql.DB, error) {
	db, err := sql.Open("sqlserver", a.SQLConf(dbName).DSN())
	if err != nil {
		return nil, fmt.Errorf("创建数据库连接失败: %w", err)
	}
	return db, nil
}

// ConnectCurrent 打开当前激活库连接（用于后续 SQL）。
func (a *App) ConnectCurrent() (*sql.DB, error) {
	return a.SQLConnect(a.CurrentDB)
}

// Line 带提示读取一行输入（自动去除尾部换行）。
func (a *App) Line(prompt string) string {
	fmt.Print(prompt)
	text, _ := a.Reader.ReadString('\n')
	return strings.TrimRight(text, "\r\n")
}

// ReadPassword 无回显读取密码。
func (a *App) ReadPassword(prompt string) string {
	fmt.Print(prompt)
	b, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return a.Line("")
	}
	return strings.TrimRight(string(b), "\r\n")
}

// Pause 按键继续
func (a *App) Pause() {
	a.Line("\n按回车键继续...")
}

// 获取可执行文件所在目录，用于确定导出文件落盘根目录。
func exeDir() string {
	exePath, err := os.Executable()
	if err != nil {
		dir, _ := os.Getwd()
		return dir
	}
	return filepath.Dir(exePath)
}

// ExportOfCurrent 将当前激活表导出到 exe 旁目录。
func (a *App) ExportOfCurrent() error {
	prefix := fmt.Sprintf("%s_%s_report", a.CurrentDB, a.CurrentTable)
	outDir := exeDir()

	db, err := a.ConnectCurrent()
	if err != nil {
		return err
	}
	defer db.Close()

	ctx := context.Background()
	res, err := exporter.Options{
		DB:             db,
		Table:          a.CurrentTable,
		ExportDir:      outDir,
		FileNamePrefix: prefix,
		CSV:            true,
		Excel:          true,
		OrderClause:    "ORDER BY 1 DESC",
	}.Run(ctx)
	if err != nil {
		return err
	}

	if res.RowCount == 0 {
		fmt.Println("\n⚠️ 数据为空，未生成报表。")
		return nil
	}

	cstat, _ := os.Stat(res.CSVPath)
	estat, _ := os.Stat(res.ExcelPath)
	if cstat != nil {
		fmt.Println("✅ [成功 1] 已生成通用 CSV 文件 (推荐直接用 Excel 打开此文件):")
		fmt.Printf("   📍 路径: %s (%.2f KB)\n", res.CSVPath, float64(cstat.Size())/1024.0)
	}
	if estat != nil {
		fmt.Println("✅ [成功 2] 已生成标准 Excel 文件:")
		fmt.Printf("   📍 路径: %s (%.2f KB)\n\n", res.ExcelPath, float64(estat.Size())/1024.0)
	}
	return nil
}

// PrintContext 打印菜单当前上下文。
func (a *App) PrintContext() {
	mode := "普通模式 [只读设计]"
	if a.IsAdmin {
		mode = "管理员模式 [完全解锁]"
	}
	fmt.Println(strings.Repeat("=", 64))
	fmt.Printf("  WinCC 数据库架构管理 (Go 版) | 权限: %s\n", mode)
	fmt.Printf("  当前库: 【%s】 | 当前表: 【%s】\n", a.CurrentDB, a.CurrentTable)
	fmt.Println(strings.Repeat("=", 64))
}

// EnterAdminRoute 提示输入管理员口令切换权限。
func (a *App) EnterAdminRoute() {
	pwd := a.ReadPassword("请输入管理员口令: ")
	if pwd == AdminPassword {
		a.IsAdmin = true
		fmt.Println("\n🔓 身份验证通过！已成功解锁管理员架构设计模式。")
	} else {
		fmt.Println("\n❌ 密码错误，拒绝开启管理员模式！")
	}
}

// askChoice 读取并校验一个菜单序号。
func (a *App) askChoice(min, max int) int {
	for {
		in := a.Line("请输入菜单编号: ")
		if n, err := strconv.Atoi(in); err == nil && n >= min && n <= max {
			return n
		}
		fmt.Println("\n❌ 无效的输入编号或无权限执行该操作！")
	}
}

// Confirm 通用确认：返回 true 表示用户确认继续。
func (a *App) Confirm(question string) bool {
	answer := strings.ToLower(a.Line(question))
	return answer == "y"
}

// AdminGate 校验管理员权限；无权限时打印提示并返回 false。
func (a *App) AdminGate() bool {
	if !a.IsAdmin {
		fmt.Println("\n❌ 该操作需要管理员模式，请先解锁 [99]！")
		return false
	}
	return true
}
