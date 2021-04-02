// Package sorvor is an extremely fast, zero config ServeIndex for modern web applications.
package sorvor

import (
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/Termina1/sorvor/pkg/cert"
	"github.com/Termina1/sorvor/pkg/livereload"
	"github.com/Termina1/sorvor/pkg/logger"
	"github.com/Termina1/sorvor/pkg/pkgjson"
	"github.com/Termina1/sorvor/pkg/sorvor/plugins"
	"github.com/evanw/esbuild/pkg/api"
)

// Sorvor struct
type Sorvor struct {
	BuildOptions api.BuildOptions
	Entry        string
	Host         string
	Port         string
	Serve        bool
	Secure       bool
}

// BuildEntry builds a given entrypoint using esbuild
func (serv *Sorvor) BuildEntry(entry string) ([]string, api.BuildResult) {
	serv.BuildOptions.EntryPoints = []string{entry}
	serv.BuildOptions.Plugins = []api.Plugin{plugins.EnvPlugin, plugins.FolderResolutionPlugin}
	result := api.Build(serv.BuildOptions)
	outfiles := make([]string, len(result.OutputFiles))
	for _, file := range result.OutputFiles {
		if filepath.Ext(file.Path) != "map" {
			cwd, _ := os.Getwd()
			outfiles = append(outfiles, strings.TrimPrefix(file.Path, filepath.Join(cwd, serv.BuildOptions.Outdir)))
		}
	}
	return outfiles, result
}

// RunEntry builds an entrypoint and launches the resulting built file using node.js
func (serv *Sorvor) RunEntry(entry string) {
	var cmd *exec.Cmd
	var outfile string
	var result api.BuildResult
	wg := new(sync.WaitGroup)
	wg.Add(1)

	var onRebuild = func(result api.BuildResult) {
		if cmd != nil {
			err := cmd.Process.Signal(syscall.SIGINT)
			logger.Fatal(err, "Failed to stop", outfile)
		}
		cmd = exec.Command("node", outfile)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		err := cmd.Start()
		logger.Fatal(err, "Failed to start", outfile)
	}
	// start esbuild in watch mode
	serv.BuildOptions.Watch = &api.WatchMode{OnRebuild: onRebuild}
	outfiles, result := serv.BuildEntry(entry)
	outfile = filepath.Join(serv.BuildOptions.Outdir, outfiles[0])
	onRebuild(result)
	wg.Wait()
}

// BuildIndex walks the index.html, collect all the entries from <script...></script> and <link .../> tags
// it then runs it through esbuild and replaces the references in index.html with new paths
func (serv *Sorvor) BuildIndex(pkg *pkgjson.PkgJSON) []string {

	target := filepath.Join(serv.BuildOptions.Outdir, "index.html")

	var entries []string
	if _, err := os.Stat(serv.Entry); err != nil {
		logger.Fatal(err, "Entry file does not exist.", serv.Entry)
	}

	tmpl, err := template.New("index.html").Funcs(template.FuncMap{
		"livereload": func() template.HTML {
			if serv.Serve {
				return template.HTML(livereload.JsSnippet)
			}
			return ""
		},
		"esbuild": func(entry string, withTag bool) template.HTML {
			if serv.Serve {
				entry = filepath.Join(filepath.Dir(serv.Entry), entry)
				entries = append(entries, entry)
			} else {
				entry = filepath.Join(filepath.Dir(serv.Entry), entry)
			}
			outfiles, _ := serv.BuildEntry(entry)
			if withTag {
				result := ""
				for _, file := range outfiles {
					switch path.Ext(file) {
					case ".js":
						result += "<script src=\"" + file + "\"><script>"
					case ".css":
						result += "<link rel=\"stylesheet\" href=\"" + file + "\">"
					}
				}
				return template.HTML(result)
			} else {
				return template.HTML(outfiles[0])
			}
		},
		"copy": func(asset string) string {
			dest := filepath.Join(serv.BuildOptions.Outdir, asset)
			go func() {
				input, err := ioutil.ReadFile(filepath.Join(filepath.Dir(serv.Entry), asset))
				logger.Error(err, "Failed to copy asset", asset)
				err = os.MkdirAll(filepath.Join(serv.BuildOptions.Outdir, filepath.Dir(asset)), 0775)
				err = ioutil.WriteFile(dest, input, 0644)
				logger.Error(err, "Error Creating destination file", dest)
			}()
			return asset
		},
	}).ParseFiles(serv.Entry)
	logger.Fatal(err, "Unable to parse index.html")

	file, err := os.Create(target)
	logger.Fatal(err, "Unable to create index.html in outdir")
	defer file.Close()

	err = tmpl.Execute(file, pkg)
	logger.Fatal(err, "Unable to execute index.html")

	return entries
}

// ServeIndex launches esbuild in watch mode and live reloads all connected browsers
func (serv *Sorvor) ServeIndex(pkg *pkgjson.PkgJSON) {
	liveReload := livereload.New()
	liveReload.Start()
	wg := new(sync.WaitGroup)
	wg.Add(2)

	// start esbuild in watch mode
	go func() {
		serv.BuildOptions.Watch = &api.WatchMode{
			OnRebuild: func(result api.BuildResult) {
				if len(result.Errors) > 0 {
					// Todo: Enhance location information for error
					for _, err := range result.Errors {
						liveReload.Error("Build Error: " + err.Text + " @ " + err.Location.File)
					}
				} else {
					// send livereload message to connected clients
					liveReload.Reload()
				}
			},
		}
		serv.BuildIndex(pkg)
	}()

	// start our own Server
	go func() {
		http.Handle("/livereload", liveReload)
		http.Handle("/", serv)

		if serv.Secure {
			// generate self signed certs
			if _, err := os.Stat("key.pem"); os.IsNotExist(err) {
				cert.GenerateKeyPair(serv.Host)
			}
			logger.Info("sørvør", "ready on", logger.BlueText("https://", serv.Host, serv.Port))
			err := http.ListenAndServeTLS(serv.Port, "cert.pem", "key.pem", nil)
			logger.Error(err, "Failed to start https Server")
		} else {
			logger.Info("sørvør", "ready on", logger.BlueText("http://", serv.Host, serv.Port))
			err := http.ListenAndServe(serv.Port, nil)
			logger.Error(err, "Failed to start http Server")
		}
	}()

	wg.Wait()
}

// ServeHTTP is an http server handler for sorvor
func (serv *Sorvor) ServeHTTP(res http.ResponseWriter, request *http.Request) {
	res.Header().Set("access-control-allow-origin", "*")
	root := filepath.Join(serv.BuildOptions.Outdir, filepath.Clean(request.URL.Path))

	if stat, err := os.Stat(root); err != nil || stat.IsDir() {
		// Serve a root index when root is not found or when root is a directory
		http.ServeFile(res, request, filepath.Join(serv.BuildOptions.Outdir, "index.html"))
		return
	}

	// else just Serve the file normally...
	http.ServeFile(res, request, root)
	return
}
