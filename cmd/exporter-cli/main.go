// exporter-cli 是 WinCC 数据一键导出/清空工具（原 export_tool.go）。
//
// 通过命令行参数或 JSON 配置驱动 internal/exporter，不再把
// 密码、表名、目录等硬编码进代码。原版「连上就导出并清空」依赖
// 的静态配置改由 -config 指定。
//
// 构建:
//
//	go build -o exporter-cli.exe ./cmd/exporter-cli
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/microsoft/go-mssqldb"

	"wincc2csv/internal/exporter"
	"wincc2csv/internal/sqlserver"
)

// configFile 对应原 export_tool.go 顶部硬编码常量区。
type configFile struct {
	Server              string `json:"server"`
	Port                int    `json:"port"`
	User                string `json:"user"`
	Password            string `json:"password"`
	Database            string `json:"database"`
	TargetTable         string `json:"table"`
	ExportDir           string `json:"exportDir"`
	TruncateAfterExport bool   `json:"truncateAfterExport"`
	TimeoutSec          int    `json:"timeoutSec"`
}

func defaultConfig() configFile {
	return configFile{
		Server:              "127.0.0.1",
		Port:                1433,
		User:                "sa",
		Database:            "WinCCTable",
		TargetTable:         "WinCC_RuntimeData",
		ExportDir:           "./ExportData",
		TruncateAfterExport: true,
		TimeoutSec:          5,
	}
}

func run() error {
	var (
		configPath = flag.String("config", "", "JSON 配置文件路径 (可选，优先于命令行参数)")
		server     = flag.String("server", "", "SQL Server 地址")
		port       = flag.Int("port", 0, "端口")
		user       = flag.String("user", "", "用户名")
		password   = flag.String("password", "", "密码")
		database   = flag.String("database", "", "数据库名")
		table      = flag.String("table", "", "目标表名")
		dir        = flag.String("exportDir", "", "导出目录")
		keepData   = flag.Bool("keep", false, "导出后不清空表 (默认会清空)")
		noStdout   = flag.Bool("silent", false, "静默模式: 成功不打印任何输出 (原导出器静默行为)")
	)
	flag.Parse()

	cfg := defaultConfig()

	// 1. 外部 JSON 配置为底座；命令行参数可覆盖。
	if *configPath != "" {
		raw, err := os.ReadFile(*configPath)
		if err != nil {
			return fmt.Errorf("读取配置文件失败: %w", err)
		}
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return fmt.Errorf("解析配置文件失败: %w", err)
		}
	}

	if *server != "" {
		cfg.Server = *server
	}
	if *port > 0 {
		cfg.Port = *port
	}
	if *user != "" {
		cfg.User = *user
	}
	if *password != "" {
		cfg.Password = *password
	}
	if *database != "" {
		cfg.Database = *database
	}
	if *table != "" {
		cfg.TargetTable = *table
	}
	if *dir != "" {
		cfg.ExportDir = *dir
	}
	if *keepData {
		cfg.TruncateAfterExport = false
	}

	dbc := sqlserver.Config{
		Server:   cfg.Server,
		Port:     cfg.Port,
		User:     cfg.User,
		Password: cfg.Password,
		Database: cfg.Database,
		Timeout:  cfg.TimeoutSec,
	}

	db, err := sql.Open("sqlserver", dbc.DSN())
	if err != nil {
		return fmt.Errorf("创建数据库连接失败: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutSec)*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("数据库连接不可达: %w", err)
	}

	res, err := exporter.Options{
		DB:                  db,
		Table:               cfg.TargetTable,
		ExportDir:           cfg.ExportDir,
		TruncateAfterExport: cfg.TruncateAfterExport,
		CSV:                 true,
		Excel:               true,
	}.Run(ctx)
	if err != nil {
		return err
	}

	if *noStdout {
		return nil
	}

	if res.RowCount == 0 {
		fmt.Println("⚠️ 导出的表为空 (已跳过落盘)")
	} else {
		fmt.Printf("📊 导出完成: %d 行\n", res.RowCount)
		if _, err := os.Stat(res.ExcelPath); err == nil {
			fmt.Printf("   Excel: %s\n", res.ExcelPath)
		}
		if _, err := os.Stat(res.CSVPath); err == nil {
			fmt.Printf("   CSV  : %s\n", res.CSVPath)
		}
		if cfg.TruncateAfterExport {
			fmt.Println("💥 源表数据已清空")
		}
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "❌", err)
		os.Exit(1)
	}
}
