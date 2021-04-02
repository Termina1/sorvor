package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Termina1/sorvor/pkg/logger"
	"github.com/Termina1/sorvor/pkg/pkgjson"
	"github.com/Termina1/sorvor/pkg/sorvor"
	"github.com/evanw/esbuild/pkg/api"
	"github.com/evanw/esbuild/pkg/cli"
)

var version = "development"

func readOptions(pkgJSON *pkgjson.PkgJSON) *sorvor.Sorvor {
	var err error
	var esbuildArgs []string

	osArgs := os.Args[1:]
	serv := &sorvor.Sorvor{}
	hasOutDir := false
	hasLogLevel := false

	for _, arg := range osArgs {
		switch {
		case strings.HasPrefix(arg, "--version"):
			logger.Info("sørvør version", version)
			os.Exit(0)
		case strings.HasPrefix(arg, "--outdir"):
			hasOutDir = true
			esbuildArgs = append(esbuildArgs, arg)
		case strings.HasPrefix(arg, "--log-level"):
			hasLogLevel = true
			esbuildArgs = append(esbuildArgs, arg)
		case strings.HasPrefix(arg, "--host"):
			serv.Host = arg[len("--host="):]
		case strings.HasPrefix(arg, "--port"):
			port, err := strconv.Atoi(arg[len("--port="):])
			logger.Fatal(err, "Invalid port value")
			serv.Port = ":" + strconv.Itoa(port)
		case arg == "--serve":
			serv.Serve = true
		case arg == "--secure":
			serv.Secure = true
		case !strings.HasPrefix(arg, "--"):
			serv.Entry = arg
		default:
			esbuildArgs = append(esbuildArgs, arg)
		}
	}

	esbuildArgs = append(esbuildArgs, "--bundle")
	if !hasOutDir {
		esbuildArgs = append(esbuildArgs, "--outdir=dist")
	}
	if !hasLogLevel {
		esbuildArgs = append(esbuildArgs, "--log-level=warning")
	}

	serv.BuildOptions, err = cli.ParseBuildOptions(esbuildArgs)
	logger.Fatal(err, "Invalid option for esbuild")
	serv.BuildOptions.Write = true
	if serv.Serve == true {
		if serv.Port == "" {
			serv.Port = ":1234"
		}
		serv.BuildOptions.Define = map[string]string{"process.env.NODE_ENV": "'development'"}
	} else {
		serv.BuildOptions.Define = map[string]string{"process.env.NODE_ENV": "'production'"}
	}

	if serv.BuildOptions.Format == api.FormatDefault {
		serv.BuildOptions.Format = api.FormatESModule
	}
	if serv.Entry == "" {
		serv.Entry = "public/index.html"
	}
	if serv.Host == "" {
		serv.Host = "localhost"
	}
	if serv.BuildOptions.Platform == api.PlatformNode {
		for key := range pkgJSON.Dependencies {
			serv.BuildOptions.External = append(serv.BuildOptions.External, key)
		}
	}
	logger.Level = serv.BuildOptions.LogLevel
	return serv
}

func main() {
	var pkgJSON *pkgjson.PkgJSON
	pkg, err := ioutil.ReadFile("package.json")
	if err == nil {
		pkgJSON, err = pkgjson.Parse(pkg)
	}

	serv := readOptions(pkgJSON)

	err = os.MkdirAll(serv.BuildOptions.Outdir, 0775)
	logger.Fatal(err, "Failed to create output directory")

	if filepath.Ext(serv.Entry) != ".html" {
		if serv.Serve == true {
			serv.RunEntry(serv.Entry)
		} else {
			serv.BuildEntry(serv.Entry)
		}
	} else {
		if serv.Serve == true {
			serv.ServeIndex(pkgJSON)
		} else {
			serv.BuildIndex(pkgJSON)
		}
	}
}
