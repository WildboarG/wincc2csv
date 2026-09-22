# wincc2csv

> 一套针对 WinCC 归档数据库（SQL Server）的管理工具集：
> 交互式建表/删表/字段管理、全量数据导出 CSV/Excel、以及把 WinCC 变量列表生成 C-Script 写入动作的网页生成器。

---

## 项目结构

不再用“三个根级 `package main` 文件分别编译”的写法（那样会互相 `main redeclared` 冲突），
统一为标准 Go 布局：所有复用逻辑进 `internal/`，可执行程序放在 `cmd/` 下。

```
wincc2csv/
├── cmd/
│   ├── admin/          # 主程序（原 main.go + action.go 合体）
│   │                    默认 = 数据库管理终端；-generator = 脚本生成器页面
│   ├── scriptgen/      # 独立脚本生成器（原 action.go 单文件形态）
│   └── exporter-cli/   # 一键导出/清空（原 export_tool.go，配置外置化）
├── internal/
│   ├── sqlserver/      # DSN 连接串构造（统一）
│   ├── exporter/       # 导出 CSV/Excel 可复用引擎（含值格式化）
│   ├── generator/      # 变量→C-Script 生成核心 + Web UI
│   └── app/            # 交互式终端应用（登录/选库选表/建表/字段/清空/删表）
├── config.example.json # exporter-cli 的参数模板（请复制为 config.json 使用）
└── note/ tools/      
```

## 构建

```bash
# 1) 管理终端（含 -generator 开关）
go build -ldflags="-s -w" -o wincc_admin.exe ./cmd/admin

# 2) 独立脚本生成器（等价旧 action.exe）
go build -ldflags="-s -w" -o wincc_gen.exe ./cmd/scriptgen

# 3) 一键导出/清空（等价旧 export_tool.exe）
go build -ldflags="-s -w" -o wincc_exporter.exe ./cmd/exporter-cli
```

可选：UPX 极致压缩 `upx --best wincc_admin.exe`。

## 使用

### 管理终端（admin 双模式）

```bash
./wincc_admin.exe          # 交互式管理终端（登录→选库→选表→菜单）
./wincc_admin.exe -generator   # 跳过终端，直接打开脚本生成器浏览器页面
```


### 脚本生成器（scriptgen）

```bash
./wincc_gen.exe
# 自动打开浏览器 http://127.0.0.1:随机端口
# 在左栏粘贴 "变量名 类型" 多行列表，点「生成」得到 gscAction 完整 C 代码：
# 步骤：粘贴到 WinCC 全局动作 → 编译保存 → 设置周期/非周期触发 → 数据写入临时表
```

### 一键导出 + 清空（exporter-cli）

把 `config.example.json` 复制为 `config.json` 并按现场改好（**数据库/表名/密码/导出目录**），随后：

```bash
./wincc_exporter.exe -config config.json        # 导出到指定目录并清空表
./wincc_exporter.exe -config config.json -keep   # 只导出，不清空
# 亦可用命令行参数覆盖 (如 -server xx -database xx)
# 密码等敏感信息建议只写进 config.json（已被 .gitignore 排除）
```

导出文件命名：`<表名>_<yyyyMMdd_HHmmss>.csv / .xlsx`，CSV 自带 UTF-8 BOM，
Excel 中文不乱码。表为空时不产生文件。





### 工作原理

---
通过给WINCC所使用的数据库添加临时数据表，表内添加所需导出报表的变量。

通过生成的C动作脚本，在wincc内使用周期/条件去控制动作脚本写入临时数据表。

用go小工具导出数据表的内容并导出csv报表。