// Package exporter 提供将 SQL Server 表数据一键导出为 CSV/Excel 并可选清空表的能力。
package exporter

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/xuri/excelize/v2"
)

// FormatDBValue 将 database/sql 扫描出的值统一格式化为展示/写入字符串。
// time.Time 与 []byte 是 go-mssqldb 中最常见的两种返回，单独处理。
func FormatDBValue(val any) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case time.Time:
		return v.Format("2006-01-02 15:04:05.000")
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// Options 描述一次导出任务的参数。
type Options struct {
	DB                  *sql.DB // 已建立的数据库连接（负责 SELECT 与可选 TRUNCATE）
	Table               string  // 要导出的表名
	ExportDir           string  // 输出目录（自动递归创建）
	TruncateAfterExport bool    // 导出成功后是否清空原表
	FileNamePrefix      string  // 文件名前缀；为空时使用表名
	CSV                 bool    // 是否输出 CSV
	Excel               bool    // 是否输出 Excel
	OrderClause         string  // 可选 ORDER BY 子句
}

// Result 描述一次导出任务的产出。
type Result struct {
	CSVPath   string
	ExcelPath string
	RowCount  int
}

// Run 执行导出：读取全表 → 生成 CSV/Excel → 报告行数。
//
// 若 TruncateAfterExport 为 true，会在文件成功生成后优先 TRUNCATE，
// 失败时降级为 DELETE。数据为空时不执行清库动作。
func (o Options) Run(ctx context.Context) (*Result, error) {
	if o.Table == "" {
		return nil, fmt.Errorf("未指定导出表名")
	}
	if err := os.MkdirAll(o.ExportDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}

	prefix := o.FileNamePrefix
	if prefix == "" {
		prefix = o.Table
	}
	stamp := time.Now().Format("20060102_150405")

	res := &Result{
		CSVPath:   filepath.Join(o.ExportDir, fmt.Sprintf("%s_%s.csv", prefix, stamp)),
		ExcelPath: filepath.Join(o.ExportDir, fmt.Sprintf("%s_%s.xlsx", prefix, stamp)),
	}

	orderSQL := ""
	if o.OrderClause != "" {
		orderSQL = " " + o.OrderClause
	}
	rows, err := o.DB.QueryContext(ctx, fmt.Sprintf("SELECT * FROM [%s]%s;", o.Table, orderSQL))
	if err != nil {
		return nil, fmt.Errorf("执行数据查询失败: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("获取列结构失败: %w", err)
	}
	colCount := len(columns)

	// CSV 通道：带 UTF-8 BOM，保证 Excel 中文不乱码
	var csvWriter *csv.Writer
	if o.CSV {
		cf, err := os.Create(res.CSVPath)
		if err != nil {
			return nil, fmt.Errorf("创建 CSV 文件失败: %w", err)
		}
		defer cf.Close()
		if _, err := cf.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
			return nil, fmt.Errorf("写入 CSV BOM 失败: %w", err)
		}
		csvWriter = csv.NewWriter(cf)
		if err := csvWriter.Write(columns); err != nil {
			return nil, fmt.Errorf("写入 CSV 列头失败: %w", err)
		}
		defer csvWriter.Flush()
	}

	// Excel 通道
	var xf *excelize.File
	var sheetName string
	if o.Excel {
		xf = excelize.NewFile()
		defer func() { _ = xf.Close() }()
		sheetName = "RuntimeData"
		_ = xf.SetSheetName("Sheet1", sheetName)
		for cIdx, colName := range columns {
			cell, _ := excelize.CoordinatesToCellName(cIdx+1, 1)
			_ = xf.SetCellValue(sheetName, cell, colName)
		}
	}

	values := make([]any, colCount)
	valuePtrs := make([]any, colCount)
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("读取行记录失败: %w", err)
		}
		res.RowCount++

		csvRow := make([]string, colCount)
		for i, val := range values {
			strVal := FormatDBValue(val)
			csvRow[i] = strVal
			if xf != nil {
				cell, _ := excelize.CoordinatesToCellName(i+1, res.RowCount+1)
				_ = xf.SetCellValue(sheetName, cell, strVal)
			}
		}
		if csvWriter != nil {
			if err := csvWriter.Write(csvRow); err != nil {
				return nil, fmt.Errorf("写入 CSV 行失败: %w", err)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历数据出错: %w", err)
	}

	// 无数据：删掉空文件避免误导；跳过后续落盘与清表动作
	if res.RowCount == 0 {
		if o.CSV {
			_ = os.Remove(res.CSVPath)
		}
		if o.Excel {
			_ = os.Remove(res.ExcelPath)
		}
		return res, nil
	}

	if o.Excel && xf != nil {
		if err := xf.SaveAs(res.ExcelPath); err != nil {
			return nil, fmt.Errorf("保存 Excel 失败: %w", err)
		}
	}

	if o.TruncateAfterExport {
		// 优先 TRUNCATE；若表存在外键等无法 TRUNCATE 的情况，降级为 DELETE
		sqlTruncate := fmt.Sprintf("TRUNCATE TABLE [%s];", o.Table)
		if _, err := o.DB.ExecContext(ctx, sqlTruncate); err != nil {
			sqlDelete := fmt.Sprintf("DELETE FROM [%s];", o.Table)
			if _, errDel := o.DB.ExecContext(ctx, sqlDelete); errDel != nil {
				return nil, fmt.Errorf("清空表 [%s] 失败: %w", o.Table, errDel)
			}
		}
	}

	return res, nil
}
