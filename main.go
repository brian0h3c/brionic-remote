// Command brionic-remote is a portable, cross-platform secure connection manager.
//
// It runs a single self-contained binary that serves a local web UI in your
// browser. Saved SSH/RDP/VNC connection profiles are kept in an encrypted vault
// file that you can carry between machines (for example on a USB drive).
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/brian0h3c/brionic-remote/internal/server"
	"github.com/brian0h3c/brionic-remote/internal/vault"
)

//go:embed all:web/dist
var distFS embed.FS

func main() {
	addr := flag.String("addr", "127.0.0.1:8717", "address to listen on")
	vaultPath := flag.String("vault", defaultVaultPath(), "path to the encrypted vault file")
	noBrowser := flag.Bool("no-browser", false, "do not open the browser automatically")
	autoExit := flag.Bool("auto-exit", false, "exit when the browser tab is closed")
	flag.Parse()

	static, err := fs.Sub(distFS, "web/dist")
	if err != nil {
		log.Fatalf("frontend assets: %v", err)
	}

	v := vault.New(*vaultPath)
	srv := server.New(v, static)
	if *autoExit {
		srv.EnableAutoExit()
	}

	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	url := "http://" + *addr
	fmt.Printf("Brionic Remote\n  URL:   %s\n  Vault: %s\n", url, *vaultPath)

	if !*noBrowser {
		go func() {
			time.Sleep(300 * time.Millisecond)
			openBrowser(url)
		}()
	}

	if err := httpServer.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

// defaultVaultPath stores the vault next to the executable so the whole app
// (binary + vault) is portable and can travel on a USB drive.
func defaultVaultPath() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "brionic-remote.vault")
	}
	return "brionic-remote.vault"
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd, args = "open", []string{url}
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		cmd, args = "xdg-open", []string{url}
	}
	_ = exec.Command(cmd, args...).Start()
}
