package app

import (
	"fmt"
	"strings"
)

// TypeItem 描述建表向导中可选的一种 SQL Server 数据类型。
type TypeItem struct {
	SQLType string
	Desc    string
}

// 建表向导按数字编号展示的类型菜单。
var sqlTypeMap = map[string]TypeItem{
	"1": {"FLOAT", "32位/64位浮点数 (温度/压力/流量)"},
	"2": {"BIT", "二进制布尔开关量 (0/1，泵启停/报警)"},
	"3": {"BIGINT", "32位无符号整数/状态字Flags (避免负数溢出)"},
	"4": {"INT", "有符号32位整型 (计数器/产量)"},
	"5": {"SMALLINT", "16位整型 (短整数)"},
	"6": {"NVARCHAR(128)", "可变长文本字符串 (标签/描述)"},
	"7": {"DATETIME", "日期时间类型 (带毫秒时间戳)"},
}

// 批量粘贴时识别的常见类型别名 → 正式 SQL 类型。
var pasteAliasToSQL = map[string]string{
	"float": "FLOAT", "double": "FLOAT", "real": "FLOAT",
	"bit": "BIT", "bool": "BIT", "boolean": "BIT",
	"word": "INT", "dword": "BIGINT", "uint": "BIGINT",
	"short": "SMALLINT", "int": "INT", "long": "BIGINT",
	"string": "NVARCHAR(128)", "char": "NVARCHAR(128)",
	"datetime": "DATETIME",
}

// sqlTypeChoice 将向导的编号输入映射为类型项。
func sqlTypeChoice(n string) TypeItem {
	if item, ok := sqlTypeMap[n]; ok {
		return item
	}
	return sqlTypeMap["1"]
}

// sqlTypeMenuText 返回供建表/加字段向导展示的类型菜单文本。
func sqlTypeMenuText() string {
	var sb strings.Builder
	for i := 1; i <= len(sqlTypeMap); i++ {
		k := fmt.Sprintf("%d", i)
		item := sqlTypeMap[k]
		sb.WriteString(fmt.Sprintf("  [%s] %-16s - %s\n", k, item.SQLType, item.Desc))
	}
	return sb.String()
}
