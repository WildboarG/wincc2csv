## 局域网连接连通WINCC



### 1.  测试端口连通性

```powershell
Test-NetConnection -ComputerName <虚拟机IP> -Port 1433
```

 **若端口不通，排查虚拟机内 SQL Server 配置**

**1.修改 TCP/IP 端口配置：**



1. 按 `Win + R` 输入 `compmgmt.msc`（或 `SQLServerManager15.msc`，版本不同数字可能为 11~16）。
2. 展开 **SQL Server 配置管理器** -> **SQL Server 网络配置** -> **[你的实例名] 的协议**。
3. 双击右侧的 **TCP/IP**，切换到 **IP 地址** 选项卡。
4. 拉到最底部的 **IPAll**：
   - 将 **TCP 端口** 从 `1442` 改为 `1433`。
   - 确保 **TCP 动态端口** 这一栏是完全空白的。
5. 点击 **确定** 保存。
6. 放行防火墙

```powershell
New-NetFirewallRule -DisplayName "MSSQL 1433" -Direction Inbound -Protocol TCP -LocalPort 1433 -Action Allow
```

- **验证本地监听状态**：

  在虚拟机内运行 `netstat -ano | findstr 1433`，确认处于 `LISTENING` 状态。

- **验证方式：** 确认弹出的“必须重新启动服务，所做的更改才会生效”提示窗口。



**2.重启 SQL Server 服务：**命令行快速执行。

在当前 PowerShell 窗口中运行以下命令重启服务：

```powershell
Get-Service -Name "*MSSQL*" | Where-Object {$_.Status -eq "Running" -and $_.Name -notmatch "LAUNCHPAD|TELEMETRY|BROWSER"} | Restart-Service
```

- **验证方式：** 再次检查端口监听状态，确认输出中是否出现 `1433`：

```powershell
Get-NetTCPConnection -OwningProcess (Get-Process sqlservr).Id -State Listen | Select-Object LocalAddress, LocalPort, State
```



**3.验证 1433 端口连通性：**连通性测试。

运行测试命令：

```powershell
Test-NetConnection -ComputerName 192.168.10.244 -Port 1433
```

- **验证方式：** 输出中 `TcpTestSucceeded` 显示为 `True` 即表示修复成功。



### 2.  修改密码并关闭策略限制

```powershell
sqlcmd -E -S ".\WINCC" -Q "ALTER LOGIN [sa] WITH PASSWORD = 'HYGS3305', CHECK_POLICY = OFF, CHECK_EXPIRATION = OFF"
```

### 3. 启用 sa 账号

```powershell
sqlcmd -E -S ".\WINCC" -Q "ALTER LOGIN [sa] ENABLE"
```

### 4. 确保开启混合身份验证模式（注册表注入）

WinCC 实例默认经常处于纯 Windows 验证模式，执行这行确保开启混合验证（值 `2`）：

```powershell
sqlcmd -E -S ".\WINCC" -Q "EXEC xp_instance_regwrite N'HKEY_LOCAL_MACHINE', N'Software\Microsoft\MSSQLServer\MSSQLServer', N'LoginMode', REG_DWORD, 2"
```

### 5. 重启 WINCC 实例服务生效

```powershell
Restart-Service -Name "MSSQL`$WINCC" -Force
```

### 6. 验证登录

重启完成后，用 `sa` 账号直接验证：

```powershell
sqlcmd -U sa -P "HYGS3305" -S ".\WINCC" -Q "SELECT @@VERSION;"
```

*(如果需要通过 TCP 端口验证，可将 `-S ".\WINCC"` 替换为 `-S "127.0.0.1,1433"` 或之前看到的监听端口)*