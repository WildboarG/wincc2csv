// Package generator 负责将 WinCC 变量列表生成为全局 C 动作脚本 (gscAction)。
package generator

import (
	"fmt"
	"strings"
)

// TypeMeta 描述某种 WinCC 数据类型对应的 C 代码生成模板。
type TypeMeta struct {
	CType   string
	Prefix  string
	GetFunc string
	Fmt     string
	Cast    string
}

var typeMap = map[string]TypeMeta{
	"Float":  {CType: "float", Prefix: "fVal", GetFunc: "GetTagFloat", Fmt: "%.4f", Cast: ""},
	"Double": {CType: "double", Prefix: "dVal", GetFunc: "GetTagDouble", Fmt: "%.6lf", Cast: ""},
	"Bit":    {CType: "BOOL", Prefix: "bVal", GetFunc: "GetTagBit", Fmt: "%d", Cast: "(int)"},
	"Byte":   {CType: "BYTE", Prefix: "byVal", GetFunc: "GetTagByte", Fmt: "%u", Cast: "(unsigned int)"},
	"Word":   {CType: "WORD", Prefix: "wVal", GetFunc: "GetTagWord", Fmt: "%u", Cast: "(unsigned int)"},
	"DWord":  {CType: "DWORD", Prefix: "dwVal", GetFunc: "GetTagDWord", Fmt: "%u", Cast: "(unsigned int)"},
	"Short":  {CType: "short", Prefix: "sVal", GetFunc: "GetTagSWord", Fmt: "%d", Cast: "(int)"},
	"Long":   {CType: "long", Prefix: "lVal", GetFunc: "GetTagSDWord", Fmt: "%ld", Cast: ""},
	"String": {CType: "char", Prefix: "szVal", GetFunc: "GetTagChar", Fmt: "'%s'", Cast: ""},
}

var aliasMap = map[string]string{
	"bool": "Bit", "boolean": "Bit",
	"real": "Float", "float32": "Float",
	"float64": "Double", "int": "Short",
	"int16": "Short", "int32": "Long",
	"uint": "Word", "uint16": "Word",
	"uint32": "DWord", "string": "String",
}

// TagEntry 表示一行已解析的 WinCC 标签。
type TagEntry struct {
	Name string
	Type string
}

// ParseTags 解析用户粘贴的多行变量文本。
//
// 支持行格式: `变量名 [空格/Tab/逗号] 类型`，类型缺省时为 Float；
// 以 // 开头的行与空行会被忽略。
func ParseTags(input string) []TagEntry {
	lines := strings.Split(input, "\n")
	var entries []TagEntry

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		clean := strings.ReplaceAll(line, "\t", " ")
		clean = strings.ReplaceAll(clean, ",", " ")
		parts := strings.Fields(clean)
		if len(parts) == 0 {
			continue
		}

		tagName := parts[0]
		dType := "Float"

		if len(parts) >= 2 {
			candidate := strings.ToLower(parts[1])
			if target, ok := aliasMap[candidate]; ok {
				dType = target
			} else {
				for k := range typeMap {
					if strings.EqualFold(k, parts[1]) {
						dType = k
						break
					}
				}
			}
		}
		entries = append(entries, TagEntry{Name: tagName, Type: dType})
	}
	return entries
}

// Options 描述生成脚本所需的数据库连接与目标表。
type Options struct {
	Server  string
	DB      string
	User    string
	Pwd     string
	Table   string
	RawTags string
}

// BuildScript 依据 Options 生成完整 WinCC 全局 C 动作脚本源码。
func BuildScript(req Options) string {
	entries := ParseTags(req.RawTags)
	if len(entries) == 0 {
		return "// 错误: 未解析到有效变量，请检查输入"
	}

	var declLines []string
	var readLines []string
	sqlCols := []string{"[RecordTime]"}
	sqlFmts := []string{"GETDATE()"}
	var sqlArgs []string

	hasString := false
	totalLen := len(req.Table) + 512

	for idx, entry := range entries {
		meta, ok := typeMap[entry.Type]
		if !ok {
			meta = typeMap["Float"]
		}
		varName := fmt.Sprintf("%s%d", meta.Prefix, idx+1)
		totalLen += len(entry.Name) + 40

		if entry.Type == "String" {
			hasString = true
			declLines = append(declLines, fmt.Sprintf("    char %s[256] = \"\";", varName))
			readBlock := fmt.Sprintf(`    pszTmp = GetTagChar("%s");
    if (pszTmp != NULL) {
        strncpy(%s, pszTmp, sizeof(%s) - 1);
        %s[sizeof(%s) - 1] = '\0';
        for (i = 0; %s[i] != '\0'; i++) {
            if (%s[i] == '\'') %s[i] = '_';
        }
    }`, entry.Name, varName, varName, varName, varName, varName, varName, varName)
			readLines = append(readLines, readBlock)
		} else {
			defVal := "0"
			if entry.Type == "Bit" {
				defVal = "FALSE"
			} else if entry.Type == "Float" {
				defVal = "0.0f"
			} else if entry.Type == "Double" {
				defVal = "0.0"
			}
			declLines = append(declLines, fmt.Sprintf("    %s %s = %s;", meta.CType, varName, defVal))
			readLines = append(readLines, fmt.Sprintf("    %s = %s(\"%s\");", varName, meta.GetFunc, entry.Name))
		}

		sqlCols = append(sqlCols, fmt.Sprintf("[%s]", entry.Name))
		sqlFmts = append(sqlFmts, meta.Fmt)
		sqlArgs = append(sqlArgs, fmt.Sprintf("%s%s", meta.Cast, varName))
	}

	if hasString {
		declLines = append([]string{"    char* pszTmp = NULL;", "    int i = 0;"}, declLines...)
	}

	bufSize := 2048
	if totalLen > 1500 {
		bufSize = ((totalLen / 1024) + 1) * 1024
	}

	// 字段换行包裹
	var colLines []string
	for i := 0; i < len(sqlCols); i += 4 {
		end := i + 4
		if end > len(sqlCols) {
			end = len(sqlCols)
		}
		chunk := strings.Join(sqlCols[i:end], ", ")
		if end < len(sqlCols) {
			chunk += ", "
		}
		colLines = append(colLines, fmt.Sprintf("            \"%s\"", chunk))
	}
	colStr := strings.Join(colLines, "\n")

	// VALUES 占位符换行包裹
	var fmtLines []string
	for i := 0; i < len(sqlFmts); i += 6 {
		end := i + 6
		if end > len(sqlFmts) {
			end = len(sqlFmts)
		}
		chunk := strings.Join(sqlFmts[i:end], ", ")
		if end < len(sqlFmts) {
			chunk += ", "
		}
		fmtLines = append(fmtLines, fmt.Sprintf("            \"%s\"", chunk))
	}
	fmtStr := strings.Join(fmtLines, "\n")

	// 参数列表折行
	var argChunks []string
	for i := 0; i < len(sqlArgs); i += 4 {
		end := i + 4
		if end > len(sqlArgs) {
			end = len(sqlArgs)
		}
		argChunks = append(argChunks, strings.Join(sqlArgs[i:end], ", "))
	}
	argStr := strings.Join(argChunks, ",\n            ")

	return fmt.Sprintf(`#include "apdefap.h"

int gscAction( void )
{
    __object* pConn = NULL;
    char szConnStr[512];
    char szSql[%d];

    // 1. 定义变量
%s

    // 2. 读取 WinCC 变量
%s

    // 3. 构造数据库连接字符串（带超时限制，防止阻塞 WinCC 全局动作队列）
    sprintf(szConnStr,
            "Provider=SQLOLEDB;"
            "Data Source=%s;"
            "Initial Catalog=%s;"
            "User ID=%s;"
            "Password=%s;"
            "Connect Timeout=2;");

    // 4. 精确匹配数据库列名与值
    sprintf(szSql,
            "INSERT INTO [%s] (\r\n"
%s
            ") VALUES (\r\n"
%s
            ");",
            %s);

    // 5. 创建 ADO Connection 对象
    pConn = __object_create("ADODB.Connection");
    if (pConn == NULL)
    {
        printf("[WinCC C-Script] 错误: 创建 ADO 对象失败\r\n");
        return -1;
    }

    pConn->ConnectionTimeout = 2;

    // 6. 连接并执行插入
    pConn->Open(szConnStr);
    if (pConn->State == 1)
    {
        pConn->Execute(szSql);
    }
    else
    {
        printf("[WinCC C-Script] 错误: 连接 SQL Server 失败\r\n");
    }

    // 7. 兜底保护：只要处于打开状态就必须安全关闭，杜绝句柄与连接池泄漏
    if (pConn->State == 1)
    {
        pConn->Close();
    }

    // 8. 释放 COM 对象并清空指针
    __object_delete(pConn);
    pConn = NULL;

    return 0;
}
`, bufSize, strings.Join(declLines, "\n"), strings.Join(readLines, "\n"),
		req.Server, req.DB, req.User, req.Pwd, req.Table, colStr, fmtStr, argStr)
}
