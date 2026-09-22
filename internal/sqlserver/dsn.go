// Package sqlserver 提供 SQL Server 连接串构造等公共能力，
// 供交互式管理程序 (internal/app) 与一键导出工具 (cmd/exporter) 复用。
package sqlserver

import (
	"fmt"
	"net/url"
)

// Config 描述一个 SQL Server 连接所需的全部参数。
type Config struct {
	Server   string // 主机 / 实例
	Port     int    // 端口，默认 1433
	User     string // 登录名
	Password string // 密码
	Database string // 目标库；为空表示连接 master
	Timeout  int    // 连接超时（秒），默认 5
}

// DSN 生成 go-mssqldb 使用的连接字符串。
func (c Config) DSN() string {
	if c.Port == 0 {
		c.Port = 1433
	}
	if c.Timeout == 0 {
		c.Timeout = 5
	}

	query := url.Values{}
	if c.Database != "" {
		query.Add("database", c.Database)
	}
	query.Add("connection timeout", fmt.Sprintf("%d", c.Timeout))
	query.Add("encrypt", "disable")

	u := &url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(c.User, c.Password),
		Host:     fmt.Sprintf("%s:%d", c.Server, c.Port),
		RawQuery: query.Encode(),
	}
	return u.String()
}
