package app

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// loginHeader 登录横幅。
func loginHeader() {
	fmt.Println("\n" + strings.Repeat("=", 64))
	fmt.Println("          WinCC 数据库架构设计与管理工具 (Go 版) - 登录")
	fmt.Println(strings.Repeat("=", 64))
}

// LoginFlow 完成登录认证；成功返回 true。
func (a *App) LoginFlow() bool {
	loginHeader()

	srv := a.Line(fmt.Sprintf("请输入服务器 IP/实例名 [默认 %s]: ", a.Server))
	if srv != "" {
		a.Server = srv
	}

	portStr := a.Line(fmt.Sprintf("请输入端口号 [默认 %d]: ", a.Port))
	if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
		a.Port = p
	}

	user := a.Line(fmt.Sprintf("请输入登录用户名 [默认 %s]: ", a.User))
	if user != "" {
		a.User = user
	}

	a.Pwd = a.ReadPassword("请输入数据库密码: ")

	fmt.Println("\n🔄 正在连接 SQL Server 实例...")
	db, err := a.SQLConnect("master")
	if err != nil {
		fmt.Printf("❌ 登录失败: %v\n", err)
		return false
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var version string
	if err := db.QueryRowContext(ctx, "SELECT @@VERSION;").Scan(&version); err != nil {
		fmt.Printf("❌ 认证失败或连接不可达: %v\n", err)
		return false
	}

	fmt.Println("🟢 身份认证成功！已建立主连接。")
	return true
}

// listUserDatabases 返回非系统用户库。
func (a *App) listUserDatabases() ([]string, error) {
	db, err := a.SQLConnect("master")
	if err != nil {
		return nil, fmt.Errorf("获取数据库列表连接失败: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT name FROM sys.databases 
		WHERE name NOT IN ('master', 'tempdb', 'model', 'msdb')
		  AND state_desc = 'ONLINE'
		ORDER BY name ASC;`)
	if err != nil {
		return nil, fmt.Errorf("获取数据库列表失败: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err == nil {
			names = append(names, n)
		}
	}
	return names, rows.Err()
}

// SelectDatabaseFlow 选择当前激活库；成功返回 true。
func (a *App) SelectDatabaseFlow() bool {
	dbs, err := a.listUserDatabases()
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		return false
	}
	if len(dbs) == 0 {
		fmt.Println("⚠️ 该实例下暂无用户数据库！")
		return false
	}

	fmt.Println("\n---------------- 发现以下数据库 ----------------")
	for idx, dbName := range dbs {
		fmt.Printf("  [%d] %s\n", idx+1, dbName)
	}
	fmt.Println("------------------------------------------------")

	for {
		choice := a.Line(fmt.Sprintf("请选择要进入的数据库 [1-%d] (输入 0 退出): ", len(dbs)))
		if choice == "0" {
			return false
		}
		idx, err := strconv.Atoi(choice)
		if err == nil && idx >= 1 && idx <= len(dbs) {
			a.CurrentDB = dbs[idx-1]
			fmt.Printf("🎯 当前激活数据库: 【%s】\n", a.CurrentDB)
			return true
		}
		fmt.Println("❌ 输入序号有误，请重新输入！")
	}
}

// SelectTableFlow 选择当前操作表；成功返回 true。
func (a *App) SelectTableFlow() bool {
	db, err := a.ConnectCurrent()
	if err != nil {
		fmt.Printf("❌ 连接数据库失败: %v\n", err)
		return false
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT TABLE_NAME 
		FROM INFORMATION_SCHEMA.TABLES 
		WHERE TABLE_TYPE = 'BASE TABLE' 
		  AND TABLE_NAME NOT IN ('sysdiagrams')
		ORDER BY TABLE_NAME ASC;`)
	if err != nil {
		fmt.Printf("❌ 获取表清单失败: %v\n", err)
		return false
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			tables = append(tables, name)
		}
	}

	if len(tables) == 0 {
		fmt.Printf("⚠️ 数据库 [%s] 中没有可操作的用户表！\n", a.CurrentDB)
		createNow := strings.ToLower(a.Line("是否立即创建新表? (y/N): "))
		if createNow == "y" {
			if !a.IsAdmin {
				pwd := a.ReadPassword("新建表需要管理员权限，请输入口令: ")
				if pwd == AdminPassword {
					a.IsAdmin = true
				} else {
					fmt.Println("❌ 口令错误，建表取消！")
					return false
				}
			}
			a.CreateNewTable()
			return a.CurrentTable != ""
		}
		return false
	}

	fmt.Printf("\n---------------- [%s] 表清单 ----------------\n", a.CurrentDB)
	for idx, tbl := range tables {
		fmt.Printf("  [%d] %s\n", idx+1, tbl)
	}
	fmt.Println("------------------------------------------------")

	for {
		choice := a.Line(fmt.Sprintf("请选择操作表编号 [1-%d] (输入 0 返回): ", len(tables)))
		if choice == "0" {
			return false
		}
		idx, err := strconv.Atoi(choice)
		if err == nil && idx >= 1 && idx <= len(tables) {
			a.CurrentTable = tables[idx-1]
			fmt.Printf("🎯 当前激活操作表: 【%s】\n", a.CurrentTable)
			return true
		}
		fmt.Println("❌ 输入序号有误，请重新输入！")
	}
}

// queryRows 辅助执行查询。
func (a *App) queryRows(sqlText string, args ...any) (*sql.Rows, error) {
	if a.CurrentDB == "" {
		return nil, fmt.Errorf("尚未选择数据库")
	}
	db, err := a.ConnectCurrent()
	if err != nil {
		return nil, err
	}
	return db.Query(sqlText, args...)
}

// ShowTableSchemaCmd 菜单入口封装。
func (a *App) ShowTableSchemaCmd() {
	if a.CurrentTable == "" {
		fmt.Println("⚠️ 尚未选择表！")
		return
	}
	a.ShowTableSchema()
}
