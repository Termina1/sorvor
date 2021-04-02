package plugins

import (
	"os"
	"path"

	"github.com/evanw/esbuild/pkg/api"
)

// EnvPlugin reads environment variable
var FolderResolutionPlugin = api.Plugin{
	Name: "folder_resolution",
	Setup: func(build api.PluginBuild) {
		build.OnResolve(api.OnResolveOptions{Filter: `^src\/`},
			func(args api.OnResolveArgs) (api.OnResolveResult, error) {
				pwd, err := os.Getwd()
				ext := path.Ext(args.Path)
				if ext == "" {
					ext = ".js"
				} else {
					ext = ""
				}
				return api.OnResolveResult{
					Path: path.Join(pwd, args.Path) + ext,
				}, err
			})
		build.OnResolve(api.OnResolveOptions{Filter: `.*`},
			func(args api.OnResolveArgs) (api.OnResolveResult, error) {
				info, err := os.Stat(path.Join(args.ResolveDir, args.Path))
				if !os.IsNotExist(err) && err != nil {
					return api.OnResolveResult{}, err
				}
				if info != nil && info.IsDir() && path.Ext(args.Path) == "" {
					return api.OnResolveResult{
						Path: path.Join(args.ResolveDir, args.Path, path.Base(args.Path)+".js"),
					}, nil
				}
				return api.OnResolveResult{}, nil
			})
	},
}
