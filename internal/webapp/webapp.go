package webapp
import("embed";"io/fs";"net/http";"tialloystudio/internal/app";"tialloystudio/internal/httpapi")
//go:embed static/*
var assets embed.FS
func New(state *app.State) http.Handler{api:=httpapi.NewHandler(state);sub,_:=fs.Sub(assets,"static");files:=http.FileServer(http.FS(sub));index,_:=fs.ReadFile(sub,"index.html");return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if len(r.URL.Path)>=5&&r.URL.Path[:5]=="/api/"{api.ServeHTTP(w,r);return};if r.URL.Path=="/"{w.Header().Set("Content-Type","text/html; charset=utf-8");_,_=w.Write(index);return};files.ServeHTTP(w,r)})}
