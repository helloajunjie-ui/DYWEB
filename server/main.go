package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

//go:embed all:webdist
var webFS embed.FS

func main() {
	port := flag.Int("port", 0, "覆盖监听端口")
	rootFlag := flag.String("root", "", "工作目录（默认 exe 所在目录）")
	flag.Parse()

	root := *rootFlag
	if root == "" {
		exe, err := os.Executable()
		if err == nil {
			root = filepath.Dir(exe)
		} else {
			root = "."
		}
		if isTempBuild(root) {
			root = "E:\\DYweb"
		}
	}
	absRoot, _ := filepath.Abs(root)
	_ = os.MkdirAll(filepath.Join(absRoot, "data"), 0o755)

	app := newApp(absRoot)
	listenPort := app.cfg.Get().Port
	if *port != 0 {
		listenPort = *port
	}

	mux := http.NewServeMux()
	app.registerAPI(mux)
	app.registerIngest(mux)

	mux.HandleFunc("/ws/client", app.hub.HandleWS)
	mux.Handle("/sfx/", http.StripPrefix("/sfx/", http.FileServer(http.Dir(app.sfxDir))))

	sub, err := fs.Sub(webFS, "webdist")
	if err != nil {
		log.Fatalf("前端资源缺失：%v", err)
	}
	fileServer := http.FileServer(http.FS(sub))
	servePage := func(w http.ResponseWriter, name string) {
		b, err := fs.ReadFile(sub, name)
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<h1>前端未构建</h1><p>请在 web 目录执行 npm install && npm run build</p>"))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	}

	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p := path.Clean(r.URL.Path)
		switch p {
		case "/", "/index.html":
			servePage(w, "index.html")
			return
		case "/overlay", "/overlay.html":
			servePage(w, "overlay.html")
			return
		case "/panel", "/panel.html":
			servePage(w, "panel.html")
			return
		}
		// 静态资源
		if strings.HasPrefix(p, "/assets/") {
			fileServer.ServeHTTP(w, r)
			return
		}
		// 其它路径当作静态文件尝试，失败回落 index
		fileServer.ServeHTTP(w, r)
	})

	// 每秒推一次统计，让控制台不用轮询
	go func() {
		ticker := time.NewTicker(time.Second)
		for range ticker.C {
			app.hub.Broadcast("stats", app.store.Snapshot())
		}
	}()

	// 画像脏数据定期落盘
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		for range ticker.C {
			app.store.SaveFansIfDirty()
		}
	}()

	addr := fmt.Sprintf("0.0.0.0:%d", listenPort)
	log.Printf("DYweb 场控中枢启动：http://127.0.0.1:%d  控制台 /  舞台层 /overlay  推送 /push", listenPort)
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("监听失败：%v（端口被占用可改 data/config.json 的 port 或用 -port 指定）", err)
	}
}

// isTempBuild go run 时 exe 目录在临时目录，回落到项目目录
func isTempBuild(p string) bool {
	return strings.Contains(strings.ToLower(p), "temp")
}
