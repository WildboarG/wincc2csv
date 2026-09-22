package app

import (
	"fmt"
	"strconv"
	"strings"

	"wincc2csv/internal/exporter"
)

// ShowTableSchema 展示当前表字段结构。
func (a *App) ShowTableSchema() {
	sqlStr := `
	SELECT 
		ORDINAL_POSITION,
		COLUMN_NAME,
		DATA_TYPE,
		ISNULL(CHARACTER_MAXIMUM_LENGTH, -1),
		IS_NULLABLE
	FROM INFORMATION_SCHEMA.COLUMNS 
	WHERE TABLE_NAME = @p1 
	ORDER BY ORDINAL_POSITION ASC;`

	db, err := a.ConnectCurrent()
	if err != nil {
		fmt.Printf("❌ 连接失败: %v\n", err)
		return
	}
	defer db.Close()

	rows, err := db.Query(sqlStr, a.CurrentTable)
	if err != nil {
		fmt.Printf("❌ 获取字段结构失败: %v\n", err)
		return
	}
	defer rows.Close()

	fmt.Println("\n" + strings.Repeat("=", 20) + fmt.Sprintf(" 表 [%s] 字段结构设计 ", a.CurrentTable) + strings.Repeat("=", 20))
	fmt.Printf("%-6s %-32s %-16s %-10s %-10s\n", "序号", "列名/字段", "类型", "最大长度", "允许NULL")
	fmt.Println(strings.Repeat("-", 80))

	for rows.Next() {
		var pos, maxLen int
		var colName, dataType, isNull string
		if err := rows.Scan(&pos, &colName, &dataType, &maxLen, &isNull); err == nil {
			maxLenStr := "-"
			if maxLen != -1 {
				maxLenStr = strconv.Itoa(maxLen)
			}
			fmt.Printf("%-6d %-32s %-16s %-10s %-10s\n", pos, colName, dataType, maxLenStr, isNull)
		}
	}
	_ = rows.Err()
	fmt.Println(strings.Repeat("=", 80) + "\n")
}

// PreviewRecentRecords 在终端预览指定条数最新数据。
func (a *App) PreviewRecentRecords(limit int) {
	db, err := a.ConnectCurrent()
	if err != nil {
		fmt.Printf("❌ 连接失败: %v\n", err)
		return
	}
	defer db.Close()

	// 检测排序列
	var timeCol string
	_ = db.QueryRow(`
		SELECT TOP 1 COLUMN_NAME 
		FROM INFORMATION_SCHEMA.COLUMNS 
		WHERE TABLE_NAME = @p1 AND LOWER(COLUMN_NAME) IN ('recordtime', 'timestamp', 'time', 'datetime');`,
		a.CurrentTable).Scan(&timeCol)

	orderClause := "1 DESC"
	if timeCol != "" {
		orderClause = fmt.Sprintf("[%s] DESC", timeCol)
	}

	querySQL := fmt.Sprintf("SELECT TOP (%d) * FROM [%s] ORDER BY %s;", limit, a.CurrentTable, orderClause)
	rows, err := db.Query(querySQL)
	if err != nil {
		fmt.Printf("❌ 读取最新数据失败: %v\n", err)
		return
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		fmt.Printf("❌ 获取列失败: %v\n", err)
		return
	}
	colCount := len(cols)

	fmt.Println("\n" + strings.Repeat("=", 25) + fmt.Sprintf(" 最近数据 (排序: %s) ", orderClause) + strings.Repeat("=", 25))
	for _, c := range cols {
		fmt.Printf("%-20s", c)
	}
	fmt.Println("\n" + strings.Repeat("-", colCount*20))

	values := make([]any, colCount)
	valuePtrs := make([]any, colCount)
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	count := 0
	for rows.Next() {
		count++
		_ = rows.Scan(valuePtrs...)
		for _, val := range values {
			strVal := exporter.FormatDBValue(val)
			if strVal == "" {
				strVal = "NULL"
			}
			if len(strVal) > 18 {
				strVal = strVal[:15] + "..."
			}
			fmt.Printf("%-20s", strVal)
		}
		fmt.Println()
	}

	if count == 0 {
		fmt.Printf("⚠️ 表 [%s] 暂无任何数据记录！\n", a.CurrentTable)
	}
	fmt.Println(strings.Repeat("=", colCount*20) + "\n")
}

// readTypeChoice 通用读取类型编号。
func (a *App) readTypeChoice(prompt string) (TypeItem, bool) {
	fmt.Println("\n请选择数据类型:")
	fmt.Print(sqlTypeMenuText())
	choice := a.Line(prompt)
	item, ok := sqlTypeMap[choice]
	if !ok {
		fmt.Println("❌ 无效的类型选择！")
		return TypeItem{}, false
	}
	return item, true
}

// CreateNewTable 创建新表；成功返回 true。
func (a *App) CreateNewTable() bool {
	fmt.Println("\n" + strings.Repeat("=", 25) + " 新建 WinCC 数据表 " + strings.Repeat("=", 25))
	newTableName := a.Line("请输入新建表名 [默认 WinCC_RuntimeData]: ")
	if newTableName == "" {
		newTableName = "WinCC_RuntimeData"
	}

	db, err := a.ConnectCurrent()
	if err != nil {
		fmt.Printf("❌ 连接失败: %v\n", err)
		return false
	}
	defer db.Close()

	var existsCount int
	_ = db.QueryRow("SELECT COUNT(1) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = @p1;", newTableName).Scan(&existsCount)
	if existsCount > 0 {
		fmt.Printf("❌ 表 [%s] 已经存在！\n", newTableName)
		return false
	}

	fmt.Println("\n选择建表字段录入模式:")
	fmt.Println("  [1] 批量粘贴模式 (推荐：直接从 Excel 粘贴变量名和类型，或仅粘贴变量名)")
	fmt.Println("  [2] 交互式手动录入模式 (逐个输入字段与选择类型)")
	subChoice := a.Line("请选择录入方式 [1/2, 默认 1]: ")

	colDefs := []string{
		"[Id] BIGINT IDENTITY(1,1) PRIMARY KEY",
		"[RecordTime] DATETIME DEFAULT (GETDATE()) NOT NULL",
	}

	if subChoice == "2" {
		fmt.Println("\n系统已默认内置: [Id] (主键自增), [RecordTime] (归档时间戳)")
		for {
			colName := a.Line("\n请输入字段名 (直接回车结束录入): ")
			if colName == "" {
				break
			}
			item, ok := a.readTypeChoice(fmt.Sprintf("类型编号 [1-%d, 默认 1-FLOAT]: ", len(sqlTypeMap)))
			if !ok {
				item = sqlTypeMap["1"]
			}
			colDefs = append(colDefs, fmt.Sprintf("[%s] %s NULL", colName, item.SQLType))
			fmt.Printf("已添加: [%s] %s\n", colName, item.SQLType)
		}
	} else {
		fmt.Println("\n请输入或粘贴变量行 (支持 Tab/逗号/空格，格式: 变量名 类型；若不写类型默认为 FLOAT)")
		fmt.Println("输入完成后，在新的一行输入 'END' 并按回车提交:")
		fmt.Println(strings.Repeat("-", 50))

		var pastedLines []string
		for {
			line := a.Line("")
			if strings.EqualFold(strings.TrimSpace(line), "END") {
				break
			}
			if strings.TrimSpace(line) != "" {
				pastedLines = append(pastedLines, strings.TrimSpace(line))
			}
		}

		if len(pastedLines) == 0 {
			fmt.Println("⚠️ 未输入任何变量，建表取消！")
			return false
		}

		for _, raw := range pastedLines {
			clean := strings.ReplaceAll(raw, "\t", " ")
			clean = strings.ReplaceAll(clean, ",", " ")
			parts := strings.Fields(clean)
			cName := parts[0]
			cType := "FLOAT"
			if len(parts) >= 2 {
				candidate := strings.ToLower(parts[1])
				if mapped, ok := pasteAliasToSQL[candidate]; ok {
					cType = mapped
				}
			}
			colDefs = append(colDefs, fmt.Sprintf("[%s] %s NULL", cName, cType))
		}
	}

	colsSQL := strings.Join(colDefs, ",\n            ")
	createSQL := fmt.Sprintf(`
	CREATE TABLE [%s] (
            %s
	);
	CREATE NONCLUSTERED INDEX [IX_%s_RecordTime] 
	ON [%s]([RecordTime] DESC);`, newTableName, colsSQL, newTableName, newTableName)

	if _, err := db.Exec(createSQL); err != nil {
		fmt.Printf("\n❌ [建表失败]: %v\n", err)
		return false
	}

	fmt.Printf("\n🎉 [成功] 数据表 [%s] 创建完成！已自动创建 RecordTime 降序索引。\n", newTableName)
	a.CurrentTable = newTableName
	a.ShowTableSchema()
	return true
}

// AddTableColumn 新增字段。
func (a *App) AddTableColumn() {
	colName := a.Line("请输入要新增的字段名称 (支持包含 # 和中文): ")
	if colName == "" {
		fmt.Println("❌ 字段名不能为空！")
		return
	}

	item, ok := a.readTypeChoice(fmt.Sprintf("请输入类型编号 [1-%d]: ", len(sqlTypeMap)))
	if !ok {
		return
	}

	alterSQL := fmt.Sprintf("ALTER TABLE [%s] ADD [%s] %s NULL;", a.CurrentTable, colName, item.SQLType)

	db, err := a.ConnectCurrent()
	if err != nil {
		fmt.Printf("❌ 连接失败: %v\n", err)
		return
	}
	defer db.Close()

	if _, err := db.Exec(alterSQL); err != nil {
		fmt.Printf("❌ [失败] 添加字段错误: %v\n", err)
		return
	}

	fmt.Printf("✅ [成功] 字段 [%s] (%s) 已添加至表 [%s]！\n", colName, item.SQLType, a.CurrentTable)
	a.ShowTableSchema()
}

// DropTableColumn 删除字段（保护 Id/RecordTime）。
func (a *App) DropTableColumn() {
	a.ShowTableSchema()
	colName := a.Line("⚠️ 请输入要彻底删除的字段名称 (区分大小写): ")
	if colName == "" {
		return
	}

	lower := strings.ToLower(colName)
	if lower == "id" || lower == "recordtime" {
		fmt.Printf("🛑 [保护拦截] 字段 [%s] 为核心系统字段，禁止删除！\n", colName)
		return
	}

	confirm := strings.ToLower(a.Line(fmt.Sprintf("🚨 危险操作：确认彻底删除表 [%s] 的字段 [%s] 吗? (y/N): ", a.CurrentTable, colName)))
	if confirm != "y" {
		fmt.Println("已取消删除字段操作。")
		return
	}

	dropSQL := fmt.Sprintf("ALTER TABLE [%s] DROP COLUMN [%s];", a.CurrentTable, colName)

	db, err := a.ConnectCurrent()
	if err != nil {
		fmt.Printf("❌ 连接失败: %v\n", err)
		return
	}
	defer db.Close()

	if _, err := db.Exec(dropSQL); err != nil {
		fmt.Printf("❌ [失败] 删除字段失败 (若字段存在约束/索引需先解绑): %v\n", err)
		return
	}

	fmt.Printf("✅ [成功] 字段 [%s] 已从表 [%s] 中完全移除！\n", colName, a.CurrentTable)
	a.ShowTableSchema()
}

// TruncateCurrentTable 清空当前表。
func (a *App) TruncateCurrentTable() {
	fmt.Println("\n" + strings.Repeat("🚨 ", 20))
	fmt.Printf("【高危操作警告】你正在尝试清空数据表: [%s].[dbo].[%s]！\n", a.CurrentDB, a.CurrentTable)
	fmt.Println("执行后表中所有数据将被彻底抹除，ID 自增计数器将重置为 1，且无法回滚！")
	fmt.Println(strings.Repeat("🚨 ", 20))

	confirm := a.Line("若确认清空，请输入大写的 'CLEAR' 确认 (其他任意字符取消): ")
	if confirm != "CLEAR" {
		fmt.Println("操作已终止，数据保持不变。")
		return
	}

	db, err := a.ConnectCurrent()
	if err != nil {
		fmt.Printf("❌ 连接失败: %v\n", err)
		return
	}
	defer db.Close()

	sqlTrunc := fmt.Sprintf("TRUNCATE TABLE [%s];", a.CurrentTable)
	sqlDelete := fmt.Sprintf("DELETE FROM [%s];", a.CurrentTable)

	if _, err := db.Exec(sqlTrunc); err == nil {
		fmt.Printf("\n💥 [成功] 表 [%s] 数据已全部清空 (TRUNCATE 完成，ID已重置)！\n", a.CurrentTable)
	} else {
		if _, errDel := db.Exec(sqlDelete); errDel == nil {
			fmt.Printf("\n💥 [成功] 表 [%s] 数据已全部清空 (DELETE 完成)！\n", a.CurrentTable)
		} else {
			fmt.Printf("\n❌ [失败] 清空数据表出错: %v\n", errDel)
		}
	}
}

// DropCurrentTable 高危：彻底删除当前表及其数据、索引。
func (a *App) DropCurrentTable() {
	fmt.Println("\n" + strings.Repeat("🔥 ", 20))
	fmt.Printf("【最高危操作警告】你正在尝试彻底删除数据表: [%s].[dbo].[%s]\n", a.CurrentDB, a.CurrentTable)
	fmt.Println("整张表的结构、数据以及全部列索引将被永久移除，且无法恢复！")
	fmt.Println("建议先执行 [2] 导出全量数据到 Excel 备份后再执行本操作。")
	fmt.Println(strings.Repeat("🔥 ", 20))

	// 第一步输入表名
	confirmName := a.Line(fmt.Sprintf("请输入要删除的表名 [%s] 以继续: ", a.CurrentTable))
	if strings.TrimSpace(confirmName) != a.CurrentTable {
		fmt.Println("取消：表名不匹配，操作已终止，未做任何改动。")
		return
	}

	// 第二步输入二次强确认码
	code := a.Line("这是不可逆操作，请输入大写 'DELETE TABLE' 进行最终确认: ")
	if code != "DELETE TABLE" {
		fmt.Println("取消：未输入确认码，操作已终止，未做任何改动。")
		return
	}

	db, err := a.ConnectCurrent()
	if err != nil {
		fmt.Printf("❌ 连接失败: %v\n", err)
		return
	}
	defer db.Close()

	sqlDrop := fmt.Sprintf("DROP TABLE [%s];", a.CurrentTable)
	if _, err := db.Exec(sqlDrop); err != nil {
		fmt.Printf("❌ [失败] 删除表 [%s] 出错: %v\n", a.CurrentTable, err)
		return
	}

	fmt.Printf("\n💥 [成功] 表 [%s] 已彻底删除 (结构与数据全部移除)！\n", a.CurrentTable)
	a.CurrentTable = ""
}
