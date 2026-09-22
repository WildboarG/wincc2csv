package generator

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

// Request 与网页表单字段一致，是 BuildScript 的 JSON 载体。
type Request struct {
	Server  string `json:"server"`
	DB      string `json:"db"`
	User    string `json:"user"`
	Pwd     string `json:"pwd"`
	Table   string `json:"table"`
	RawTags string `json:"rawTags"`
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// Run 启动本地 HTTP 页面并在默认浏览器中打开。
// 阻塞直到服务器关闭；用户关闭窗口/终端后整个进程退出。
func Run() error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("启动监听失败: %w", err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(pageHTML))
	})
	mux.HandleFunc("/api/generate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		result := BuildScript(Options{
			Server:  req.Server,
			DB:      req.DB,
			User:    req.User,
			Pwd:     req.Pwd,
			Table:   req.Table,
			RawTags: req.RawTags,
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"code": result})
	})

	fmt.Println("==================================================")
	fmt.Printf("WinCC 脚本生成器已就绪，正在打开界面: %s\n", url)
	fmt.Println("使用完成后直接关闭当前终端窗口即可退出。")
	fmt.Println("==================================================")

	go func() {
		time.Sleep(250 * time.Millisecond)
		openBrowser(url)
	}()

	return http.Serve(listener, mux)
}

const pageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<title>WinCC 数据归档 C 动作生成器</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Microsoft YaHei", sans-serif; background: #f0f2f5; padding: 16px; color: #333; }
  .card { background: #fff; border-radius: 8px; padding: 14px 18px; margin-bottom: 12px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 12px; }
  label { display: block; font-size: 13px; font-weight: bold; margin-bottom: 4px; color: #444; }
  input[type="text"], input[type="password"], textarea {
    width: 100%; padding: 8px 10px; border: 1px solid #d9d9d9; border-radius: 4px; font-size: 13px; outline: none; transition: border 0.2s;
  }
  input:focus, textarea:focus { border-color: #1890ff; }
  .content-split { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; height: calc(100vh - 200px); min-height: 480px; }
  .panel { display: flex; flex-direction: column; height: 100%; }
  textarea { flex: 1; resize: none; font-family: "Consolas", "Courier New", monospace; font-size: 13px; line-height: 1.5; }
  .btn-bar { display: flex; gap: 10px; margin-top: 8px; }
  button {
    padding: 8px 16px; border: none; border-radius: 4px; font-size: 13px; cursor: pointer; font-weight: bold; transition: background 0.2s;
  }
  .btn-primary { background: #1890ff; color: #fff; flex: 2; }
  .btn-primary:hover { background: #40a9ff; }
  .btn-secondary { background: #e5e5e5; color: #333; flex: 1; }
  .btn-secondary:hover { background: #d4d4d4; }
  .btn-copy { background: #52c41a; color: #fff; width: 100%; margin-top: 8px; }
  .btn-copy:hover { background: #73d13d; }
  .hint { font-size: 12px; color: #888; margin-bottom: 6px; }
</style>
</head>
<body>
<div class="card">
  <div class="grid">
    <div><label>SQL 实例 IP</label><input type="text" id="server" value="127.0.0.1"></div>
    <div><label>数据库名</label><input type="text" id="db" value="WinCCTable"></div>
    <div><label>目标数据表</label><input type="text" id="table" value="WinCC_RuntimeData"></div>
    <div><label>用户名</label><input type="text" id="user" value="sa"></div>
    <div><label>密码</label><input type="password" id="pwd" value="123456"></div>
  </div>
</div>

<div class="content-split">
  <div class="card panel">
    <label>变量列表（支持从 Excel 直接复制两列粘贴）</label>
    <div class="hint">格式: 变量名 [空格/Tab] 类型 (可选: Float, Double, Bit, Word, DWord, Long, String 等，默认 Float)</div>
    <textarea id="rawTags">Iba_HMI_1#主机电流A	Float
Iba_HMI_2#主机电流A	Float
Iba_HMI_上辊速度Rpm	Float
Iba_HMI_下辊速度Rpm	Float
Iba_HMI_轧制记录中	Bit
Iba_BatchID	String
Iba_AlarmCode	DWord</textarea>
    <div class="btn-bar">
      <button class="btn-primary" onclick="generate()">▶ 立即生成 C 代码</button>
      <button class="btn-secondary" onclick="document.getElementById('rawTags').value=''">清空输入</button>
    </div>
  </div>

  <div class="card panel">
    <label>生成的 WinCC 全局 C 脚本 (gscAction)</label>
    <div class="hint">带连接状态双重防卫与句柄自毁，直接粘贴投产</div>
    <textarea id="output" readonly style="background:#f9f9f9;"></textarea>
    <button class="btn-copy" onclick="copyCode()">复制全部代码</button>
  </div>
</div>

<script>
async function generate() {
  const payload = {
    server: document.getElementById('server').value,
    db: document.getElementById('db').value,
    table: document.getElementById('table').value,
    user: document.getElementById('user').value,
    pwd: document.getElementById('pwd').value,
    rawTags: document.getElementById('rawTags').value
  };
  const res = await fetch('/api/generate', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload)
  });
  const data = await res.json();
  document.getElementById('output').value = data.code;
}

function copyCode() {
  const out = document.getElementById('output');
  if(!out.value) return;
  out.select();
  navigator.clipboard.writeText(out.value);
  alert("代码已复制到剪贴板！");
}
window.onload = generate;
</script>
</body>
</html>`
