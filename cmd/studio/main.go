package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"tialloystudio/internal/app"
	"tialloystudio/internal/studio"
	"tialloystudio/internal/webapp"
)

func openBrowser(url string) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", "start", "", "msedge", "--app="+url)
		if err := cmd.Start(); err == nil { return }
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	} else if runtime.GOOS == "darwin" { cmd = exec.Command("open", url) } else { cmd = exec.Command("xdg-open", url) }
	_ = cmd.Start()
}
func runSmoke(path string) int { result,err:=studio.ScientificSmoke(); payload:=map[string]any{"result":result}; if err!=nil{payload["error"]=err.Error()}; data,_:=json.MarshalIndent(payload,"","  "); if path!=""{if e:=os.WriteFile(path,data,0644);e!=nil{fmt.Fprintln(os.Stderr,e);return 2}}else{fmt.Println(string(data))}; if err!=nil||result.Status!="PASS"{return 1};return 0 }
func manualFile(w http.ResponseWriter){exe,err:=os.Executable();if err!=nil{http.Error(w,"manual unavailable",404);return};p:=filepath.Join(filepath.Dir(exe),"TiAlloyStudio-Manual.docx");data,err:=os.ReadFile(p);if err!=nil{http.Error(w,"manual unavailable",404);return};w.Header().Set("Content-Type","application/vnd.openxmlformats-officedocument.wordprocessingml.document");w.Header().Set("Content-Disposition",`attachment; filename="TiAlloyStudio-Manual.docx"`);_,_=w.Write(data)}
func runEngineSmoke(path string) int { st:=app.NewState();r,err:=st.Build(app.BuildRequest{Module:"crystal",Phase:"alpha",NX:2,NY:2,NZ:2});payload:=map[string]any{"status":"PASS","engines":r.Engines};if err!=nil{payload["status"]="FAIL";payload["error"]=err.Error()};if len(r.Engines)<2{payload["status"]="FAIL";payload["error"]="managed engine reports missing"};for _,er:=range r.Engines{if er.Status!="PASS"{payload["status"]="FAIL";payload["error"]=fmt.Sprintf("%s: %s",er.Name,er.Message);break}};data,_:=json.MarshalIndent(payload,"","  ");if path!=""{if e:=os.WriteFile(path,data,0644);e!=nil{fmt.Fprintln(os.Stderr,e);return 2}}else{fmt.Println(string(data))};if payload["status"]!="PASS"{return 1};return 0 }
func main(){smoke:=flag.Bool("smoke-test",false,"run scientific smoke test and exit");smokeFile:=flag.String("smoke-test-file","","write native smoke-test JSON to file");engineSmokeFile:=flag.String("engine-smoke-file","","write managed-engine cross-check JSON to file");noBrowser:=flag.Bool("no-browser",false,"start server without launching browser");flag.Parse();if *smoke||*smokeFile!=""{os.Exit(runSmoke(*smokeFile))};if *engineSmokeFile!=""{os.Exit(runEngineSmoke(*engineSmokeFile))};ln,err:=net.Listen("tcp","127.0.0.1:0");if err!=nil{log.Fatal(err)};state:=app.NewState();base:=webapp.New(state);var lastBeat atomic.Int64;var seen atomic.Bool;started:=time.Now();var srv *http.Server;handler:=http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){switch r.URL.Path{case "/api/heartbeat":if r.Method!=http.MethodPost{w.WriteHeader(405);return};seen.Store(true);lastBeat.Store(time.Now().UnixNano());w.WriteHeader(204);return;case "/api/exit":if r.Method!=http.MethodPost{w.WriteHeader(405);return};w.WriteHeader(204);go func(){time.Sleep(100*time.Millisecond);_=srv.Close()}();return;case "/manual":manualFile(w);return;default:base.ServeHTTP(w,r)}});srv=&http.Server{Handler:handler,ReadHeaderTimeout:5*time.Second};go func(){t:=time.NewTicker(10*time.Second);defer t.Stop();for range t.C{if seen.Load(){if time.Since(time.Unix(0,lastBeat.Load()))>35*time.Second{_=srv.Close();return}}else if time.Since(started)>2*time.Minute{_=srv.Close();return}}}();url:="http://"+ln.Addr().String()+"/";fmt.Println("Ti Alloy Studio:",url);if !*noBrowser{go func(){time.Sleep(250*time.Millisecond);openBrowser(url)}()};if err:=srv.Serve(ln);err!=nil&&err!=http.ErrServerClosed{log.Fatal(err)}}
