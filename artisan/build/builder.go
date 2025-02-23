package build

/*
  Onix Config Manager - Artisan
  Copyright (c) 2018-Present by www.gatblau.org
  Licensed under the Apache License, Version 2.0 at http://www.apache.org/licenses/LICENSE-2.0
  Contributors to this project, hereby assign copyright in this code to the project,
  to be licensed under the same terms as the rest of the code.
*/
import (
	"archive/zip"
	"fmt"
	"github.com/gatblau/onix/artisan/core"
	"github.com/gatblau/onix/artisan/data"
	"github.com/gatblau/onix/artisan/merge"
	"github.com/gatblau/onix/artisan/registry"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/uuid"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Builder struct {
	zipWriter        *zip.Writer
	workingDir       string
	uniqueIdName     string
	repoURI          string
	commit           string
	from             string
	repoName         *core.PackageName
	buildFile        *data.BuildFile
	localReg         *registry.LocalRegistry
	shouldCopySource bool
	loadFrom         string
	env              *merge.Envar
	zip              bool // if the target is already zipped before packaging (e.g. jar, zip files, etc)
}

func NewBuilder() *Builder {
	// create the builder instance
	builder := new(Builder)
	// check the localRepo directory is there
	builder.localReg = registry.NewLocalRegistry()
	return builder
}

// Build the package
// from: the source to build, either http based git repository or local system git repository
// gitToken: if provided it is used to clone a remote repository that has authentication enabled
// name: the full name of the package to be built including the tag
// profileName: the name of the profile to be built. If empty then the default profile is built. If no default profile exists, the first profile is built.
// copy: indicates whether a copy should be made of the project files before packaging (only valid for from location in the file system)
// interactive: true if the console should survey for missing variables
// target: a specific target without relying on a build file
func (b *Builder) Build(from, fromPath, gitToken string, name *core.PackageName, profileName string, copy bool, interactive bool, target string) {
	b.from = from
	// prepare the source ready for the build
	repo := b.prepareSource(from, fromPath, gitToken, name, copy, target)
	// set the unique identifier name for both the zip file and the seal file
	b.setUniqueIdName(repo)
	// run commands
	// set the command execution directory
	execDir := b.loadFrom
	buildProfile := b.runProfile(profileName, execDir, interactive)
	// if the build target is a file or subdirectory in current folder
	if buildProfile.Target == "." || strings.HasPrefix(buildProfile.MergedTarget, "..") || strings.HasPrefix(buildProfile.MergedTarget, "/") {
		core.RaiseErr("invalid build target, target must be a file or folder under the build file\n")
	}
	// merge env with target
	mergedTarget, _ := core.MergeEnvironmentVars([]string{buildProfile.Target}, b.env.Vars, interactive)
	// set the merged target for later use
	buildProfile.MergedTarget = mergedTarget[0]
	// wait for the target to be created in the file system
	targetPath := filepath.Join(b.loadFrom, mergedTarget[0])
	core.Debug("waiting for build process to complete\n")
	waitForTargetToBeCreated(targetPath)
	// compress the target defined in the build.yaml' profile
	core.Debug("zipping target path '%s'\n", targetPath)
	b.zipPackage(targetPath)
	// creates a seal
	core.Debug("creating package seal\n")
	s, err := b.createSeal(buildProfile)
	core.CheckErr(err, "cannot create package seal")
	// add the package to the local repo
	core.Debug("adding package to local registry\n")
	b.localReg.Add(b.workDirZipFilename(), b.repoName, s)
	// cleanup all relevant folders and move package to target location
	core.Debug("performing cleanup\n")
	b.cleanUp()
}

// Run execute the specified function
func (b *Builder) Run(function string, path string, interactive bool, env *merge.Envar) {
	// if no path is specified use .
	if len(path) == 0 {
		path = "."
	}
	var localPath = path
	// if a relative path is passed
	if strings.HasPrefix(path, "http") {
		core.RaiseErr("the path must not be an http resource")
	}
	if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") || (!strings.HasPrefix(path, "/")) {
		// turn it into an absolute path
		absPath, err := filepath.Abs(path)
		if err != nil {
			log.Fatal(err)
		}
		localPath = absPath
	}
	bf, err := data.LoadBuildFile(filepath.Join(localPath, "build.yaml"))
	core.CheckErr(err, "cannot load build file")
	b.buildFile = bf
	b.runFunction(function, localPath, interactive, env)
}

// either clone a remote git repo or copy a local one onto the source folder
func (b *Builder) prepareSource(from string, fromPath string, gitToken string, tagName *core.PackageName, copy bool, target string) *git.Repository {
	var repo *git.Repository
	b.repoName = tagName
	// creates a temporary working directory
	b.workingDir = b.newWorkingDir()
	core.Debug("creating temporary working directory '%s'\n", b.workingDir)
	// if "from" is an http url
	if strings.HasPrefix(strings.ToLower(from), "http") {
		b.loadFrom = b.sourceDir(b.workingDir)
		// if a sub-folder was specified
		if len(fromPath) > 0 {
			// add it to the path
			b.loadFrom = filepath.Join(b.loadFrom, fromPath)
		}
		core.Debug("cloning build source repository '%s'\n", from)
		repo = b.cloneRepo(from, gitToken)
	} else
	// there is a local repo instead of a downloadable url
	{
		var localPath = from
		// if a relative path is passed
		if strings.HasPrefix(from, "./") || (!strings.HasPrefix(from, "/")) {
			// turn it into an absolute path
			absPath, err := filepath.Abs(from)
			if err != nil {
				log.Fatal(err)
			}
			localPath = absPath
		}
		// if the user requested a copy of the project before building it
		if copy {
			b.loadFrom = b.sourceDir(b.workingDir)
			// if a sub-folder was specified
			if len(fromPath) > 0 {
				// add it to the path
				b.loadFrom = filepath.Join(b.loadFrom, fromPath)
			}
			// copy the folder to the source directory
			err := copyFolder(from, b.sourceDir(b.workingDir))
			if err != nil {
				log.Fatal(err)
			}
			b.repoURI = localPath
		} else {
			// the working directory is the current directory
			b.loadFrom = localPath
			// if a sub-folder was specified
			if len(fromPath) > 0 {
				// add it to the path
				b.loadFrom = filepath.Join(b.loadFrom, fromPath)
			}
		}
		core.Debug("opening git repository '%s'", localPath)
		repo = b.openRepo(localPath)
	}
	// read build.yaml
	buildFilePath := filepath.Join(b.loadFrom, "build.yaml")
	core.Debug("loading build file from %s\n", buildFilePath)
	bf, err := data.LoadBuildFile(buildFilePath)
	// if it cannot find the build file
	if err != nil {
		if len(target) > 0 {
			core.WarningLogger.Printf("build not found in '%s', building content only package\n", filepath.Join(b.loadFrom, target))
			// dynamically creates one that packages anything on the build target
			bf = &data.BuildFile{
				Profiles: []*data.Profile{
					{
						Name:    "content-only",
						Default: true,
						Target:  target,
						Type:    "files",
					},
				},
			}
		} else {
			core.RaiseErr("cannot build package: no build profile exists or target folder has been specified instead")
		}
	}
	b.buildFile = bf
	return repo
}

// compress the target
func (b *Builder) zipPackage(targetPath string) {
	ignored := b.getIgnored()
	// get the target source information
	info, err := os.Stat(targetPath)
	core.CheckErr(err, "failed to retrieve target to compress: '%s'", targetPath)
	// if the target is a directory
	if info.IsDir() {
		// then zip it
		core.CheckErr(zipSource(targetPath, b.workDirZipFilename(), ignored), "failed to compress folder")
	} else {
		// if it is a file open it to check its type
		file, err := os.Open(targetPath)
		core.CheckErr(err, "failed to open target: %s", targetPath)
		// find the content type
		contentType, err := findContentType(file)
		core.CheckErr(err, "failed to find target content type")
		// if the file is not a zip file
		if contentType != "application/zip" {
			b.zip = false
			// the zip it
			core.CheckErr(zipSource(targetPath, b.workDirZipFilename(), ignored), "failed to compress file target")
			return
		} else {
			b.zip = true
			// find the file extension
			ext := filepath.Ext(targetPath)
			// if the extension is not zip (e.g. jar files)
			if ext != ".zip" {
				// rename the file to .zip - do not use os.Rename to avoid "invalid cross-device link" error if running in kubernetes
				core.CheckErr(renameFile(targetPath, b.workDirZipFilename()), "failed to rename file target to .zip extension")
				return
			}
			return
		}
	}
}

// clones a remote git LocalRegistry, it only accepts a token if authentication is required
// if the token is not provided (empty string) then no authentication is used
func (b *Builder) cloneRepo(repoUrl string, gitToken string) *git.Repository {
	b.repoURI = repoUrl
	// clone the remote repository
	opts := &git.CloneOptions{
		URL:      repoUrl,
		Progress: os.Stdout,
	}
	// if authentication token has been provided
	if len(gitToken) > 0 {
		// The intended use of a GitHub personal access token is in replace of your password
		// because access tokens can easily be revoked.
		// https://help.github.com/articles/creating-a-personal-access-token-for-the-command-line/
		opts.Auth = &http.BasicAuth{
			Username: "abc123", // yes, this can be anything except an empty string
			Password: gitToken,
		}
	}
	repo, err := git.PlainClone(b.sourceDir(b.workingDir), false, opts)
	if err != nil {
		_ = os.RemoveAll(b.workingDir)
		log.Fatal(err)
	}
	return repo
}

// opens a git LocalRegistry from the given path
func (b *Builder) openRepo(path string) *git.Repository {
	// find .git path in the current directory or any parents
	gitPath, _ := findGitPath(path)
	repo, _ := git.PlainOpen(gitPath)
	return repo
}

// cleanup all relevant folders and move package to target location
func (b *Builder) cleanUp() {
	// remove the working directory
	core.CheckErr(os.RemoveAll(b.workingDir), "failed to remove temporary build directory")
	// set the directory to empty
	b.workingDir = ""
}

// create a new working directory and return its path
func (b *Builder) newWorkingDir() string {
	// the working directory will be a build folder within the registry directory
	basePath := filepath.Join(core.RegistryPath(), "build")
	uid := uuid.New()
	folder := strings.Replace(uid.String(), "-", "", -1)[:12]
	workingDirPath := filepath.Join(basePath, folder)
	// creates a temporary working directory
	err := os.MkdirAll(workingDirPath, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	// create a sub-folder to zip
	err = os.MkdirAll(b.sourceDir(workingDirPath), os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	return workingDirPath
}

// construct a unique name for the package using the short HEAD commit hash and current time
func (b *Builder) setUniqueIdName(repo *git.Repository) {
	var hash = ""
	// if the repo is there
	if repo != nil {
		// get the commit head and add it to the unique reference
		ref, err := repo.Head()
		if err != nil {
			core.RaiseErr("the git repository exists but does not have a commit yet, you need at least one commit before continuing: this is so that a build reference with a commit head can be available within the build context")
		}
		b.commit = ref.Hash().String()
		hash = fmt.Sprintf("-%s", ref.Hash().String()[:10])
	}
	// get the current time
	t := time.Now()
	timeStamp := fmt.Sprintf("%04s%02d%02d%02d%02d%02d%s", strconv.Itoa(t.Year()), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), strconv.Itoa(t.Nanosecond())[:3])
	b.uniqueIdName = fmt.Sprintf("%s%s", timeStamp, hash)
	core.Debug("package files name is '%s'\n", b.uniqueIdName)
}

// remove files in the source folder that are specified in the .buildignore file
func (b *Builder) getIgnored() []string {
	ignoreFilename := ".buildignore"
	// retrieve the ignore file
	ignoreFileBytes, err := ioutil.ReadFile(filepath.Join(b.loadFrom, ".buildignore"))
	if err != nil {
		// assume no ignore file exists, do nothing
		return []string{}
	}
	// get the lines in the ignore file
	lines := strings.Split(string(ignoreFileBytes), "\n")
	// adds the .ignore file
	lines = append(lines, ignoreFilename)
	// turns relative paths into absolute paths
	var output []string
	for _, line := range lines {
		if !filepath.IsAbs(line) {
			line, err = filepath.Abs(line)
			if err != nil {
				core.RaiseErr("cannot convert relation path to absolute path: %s", err)
			}
		}
		output = append(output, line)
	}
	return output
}

// run a specified function
func (b *Builder) runFunction(function string, path string, interactive bool, env *merge.Envar) {
	// if in debug mode, print environment variables
	env.Debug(fmt.Sprintf("executing function: %s\n", function))
	// if inputs are defined for the function then survey for data
	i := data.SurveyInputFromBuildFile(function, b.buildFile, interactive, false, env)
	// merge the collected input with the current environment
	env.Merge(i.Env(false))
	// gets the function to run
	fx := b.buildFile.Fx(function)
	if fx == nil {
		core.RaiseErr("function %s does not exist in the build file", function)
		return
	}
	// set the unique name for the run
	b.setUniqueIdName(b.openRepo(path))
	if len(b.from) == 0 {
		b.from = path
	}
	// get the build file environment and merge any subshell command
	vars := b.evalSubshell(b.buildFile.GetEnv(), path, env, interactive)
	// add the merged vars to the env
	env = env.Append(vars)
	// get the fx environment and merge any subshell command
	vars = b.evalSubshell(fx.GetEnv(), path, env, interactive)
	// combine the current environment with the function environment
	buildEnv := env.Append(vars)
	// add build specific variables
	buildEnv = buildEnv.Append(b.getBuildEnv())
	// for each run statement in the function
	for _, cmd := range fx.Run {
		// add function level vars
		buildEnv = buildEnv.Append(fx.GetEnv())
		// if the statement has a function call
		if ok, expr, shell := core.HasShell(cmd); ok {
			out, err := Exe(shell, path, buildEnv, interactive)
			core.CheckErr(err, "cannot execute subshell command: %s", cmd)
			// merges the output of the subshell in the original command
			cmd = strings.Replace(cmd, expr, out, -1)
			// execute the statement
			err = execute(cmd, path, buildEnv, interactive)
			core.CheckErr(err, "cannot execute command: %s", cmd)
		} else if ok, fx := core.HasFunction(cmd); ok {
			// executes the function
			b.runFunction(fx, path, interactive, env)
		} else {
			// execute the statement
			err := execute(cmd, path, buildEnv, interactive)
			core.CheckErr(err, "cannot execute command: %s", cmd)
		}
	}
}

// execute all commands in the specified profile
// if not profile is specified, it uses the default profile
// if a default profile has not been defined, then uses the first profile in the build file
// returns the profile used
func (b *Builder) runProfile(profileName string, execDir string, interactive bool) *data.Profile {
	// construct an environment with the vars at build file level
	env := merge.NewEnVarFromSlice(os.Environ())
	// get the build file environment and merge any subshell command
	vars := b.evalSubshell(b.buildFile.GetEnv(), execDir, env, interactive)
	// add the merged vars to the env
	env = env.Append(vars)
	// for each build profile
	for _, profile := range b.buildFile.Profiles {
		// if a profile name has been provided then build it
		if len(profileName) > 0 && profile.Name == profileName {
			core.Debug("using build profile '%s'\n", profile.Name)
			// get the profile environment and merge any subshell command
			vars = b.evalSubshell(profile.GetEnv(), execDir, env, interactive)
			// combine the current environment with the profile environment
			buildEnv := env.Append(vars)
			// add build specific variables
			buildEnv = buildEnv.Append(b.getBuildEnv())
			// stores the build environment
			b.env = buildEnv
			core.Debug("profile variables:\n%s\n", buildEnv.String())
			// for each run statement in the profile
			for _, cmd := range profile.Run {
				// execute the statement
				if ok, expr, shell := core.HasShell(cmd); ok {
					out, err := Exe(shell, execDir, buildEnv, interactive)
					core.CheckErr(err, "cannot execute subshell command: %s", cmd)
					// merges the output of the subshell in the original command
					cmd = strings.Replace(cmd, expr, out, -1)
					// execute the statement
					core.Debug("executing profile command: %s; @ %s\n", cmd, execDir)
					err = execute(cmd, execDir, buildEnv, interactive)
					core.CheckErr(err, "cannot execute command: %s", cmd)
				} else if ok, fx := core.HasFunction(cmd); ok {
					// executes the function
					b.runFunction(fx, execDir, interactive, env)
				} else {
					// execute the statement
					core.Debug("executing profile command: %s; @ %s\n", cmd, execDir)
					err := execute(cmd, execDir, buildEnv, interactive)
					core.CheckErr(err, "cannot execute command: %s", cmd)
				}
			}
			return profile
		}
		// if the profile has not been provided
		if len(profileName) == 0 {
			// check if a default profile has been set
			defaultProfile := b.buildFile.DefaultProfile()
			// use the default profile
			if defaultProfile != nil {
				core.Debug("using default profile: %s\n", defaultProfile.Name)
				return b.runProfile(defaultProfile.Name, execDir, interactive)
			} else {
				core.Debug("using first profile: %s\n", b.buildFile.Profiles[0].Name)
				// there is no default profile defined so use the first profile
				return b.runProfile(b.buildFile.Profiles[0].Name, execDir, interactive)
			}
		}
	}
	// if we got to this point then a specific profile was requested but not defined
	// so cannot continue
	core.RaiseErr("the requested profile '%s' is not defined in Artisan's build configuration", profileName)
	return nil
}

// evaluate sub-shells and replace their values in the variables
func (b *Builder) evalSubshell(vars map[string]string, execDir string, env *merge.Envar, interactive bool) map[string]string {
	// if env is nil then create one injecting the artisan build environment variables
	if env == nil {
		env = merge.NewEnVarFromMap(b.getBuildEnv())
	} else {
		// otherwise, add the artisan build environment variables to the existing environment
		env.Merge(merge.NewEnVarFromMap(b.getBuildEnv()))
	}
	// ensures env contains the variables in vars
	env.Vars = mergeMaps(env.Vars, vars)
	for k, v := range vars {
		// merge any existing variables in the variable
		s, _ := core.MergeEnvironmentVars([]string{v}, env.Vars, false)
		// update the value with merged expression
		vars[k] = s[0]
		if ok, expr, shell := core.HasShell(v); ok {
			out, err := Exe(shell, execDir, env, interactive)
			core.CheckErr(err, "cannot execute subshell command: %s", v)
			// merges the output of the subshell in the original variable
			vars[k] = strings.Replace(v, expr, out, -1)
		}
	}
	return vars
}

// return an absolute path using the working directory as base
func (b *Builder) inWorkingDirectory(relativePath string) string {
	return filepath.Join(b.workingDir, relativePath)
}

// return an absolute path using the source directory as base
func (b *Builder) inSourceDirectory(relativePath string) string {
	return filepath.Join(b.sourceDir(b.workingDir), relativePath)
}

// create the package Seal
func (b *Builder) createSeal(profile *data.Profile) (*data.Seal, error) {
	filename := b.uniqueIdName
	// merge the labels in the profile with the ones at the build file level
	labels := mergeMaps(b.buildFile.Labels, profile.Labels)
	// gets the size of the package
	zipInfo, err := os.Stat(b.workDirZipFilename())
	if err != nil {
		return nil, err
	}
	// prepare the seal info
	info := &data.Manifest{
		Type:    profile.Type,
		License: profile.License,
		Ref:     filename,
		OS:      runtime.GOOS,
		Profile: profile.Name,
		Labels:  labels,
		Source:  b.repoURI,
		Commit:  b.commit,
		Branch:  "",
		Tag:     "",
		Target:  filepath.Base(profile.MergedTarget),
		Time:    time.Now().Format(time.RFC850),
		Size:    bytesToLabel(zipInfo.Size()),
		Zip:     b.zip,
	}
	// take the hash of the zip file and seal info combined
	s := new(data.Seal)
	// the seal needs the manifest to create a checksum
	s.Manifest = info
	var buildFile *data.BuildFile
	// check if target is a folder containing a build.yaml
	innerBuildFilePath, _ := filepath.Abs(path.Join(b.from, profile.MergedTarget, "build.yaml"))
	// check if the inner build file exists
	if _, statErr := os.Stat(innerBuildFilePath); os.IsNotExist(statErr) {
		core.Debug("cannot find a build.yaml in the target folder '%s', building content package only\n", innerBuildFilePath)
		// then it is a content only package, so creates an empty build file so the process can continue
		// without adding functions to package manifest
		buildFile = &data.BuildFile{
			Env:       map[string]string{},
			Labels:    map[string]string{},
			Input:     &data.Input{},
			Profiles:  []*data.Profile{},
			Functions: []*data.Function{},
		}
	} else {
		// load the build file
		core.Debug("loading build file from target folder '%s'\n", innerBuildFilePath)
		buildFile, err = data.LoadBuildFile(innerBuildFilePath)
		core.CheckErr(err, "cannot load build file from target folder")
	}
	// only export functions if the target contains a build.yaml
	// if the manifest contains exported functions then include the runtime
	// image that should be used to execute such functions
	if buildFile.ExportFxs() {
		core.Debug("build file exports functions\n")
		// pick the runtime at the build file level if exists
		if len(buildFile.Runtime) > 0 {
			s.Manifest.Runtime = buildFile.Runtime
		}
	}
	// add exported functions to the manifest
	for _, fx := range buildFile.Functions {
		// if the function is exported
		if fx.Export != nil && *fx.Export {
			core.Debug("adding inputs to the manifest for exported function '%s'\n", fx.Name)
			// then grab the required inputs
			s.Manifest.Functions = append(s.Manifest.Functions, &data.FxInfo{
				Name:        fx.Name,
				Description: fx.Description,
				Input:       data.SurveyInputFromBuildFile(fx.Name, buildFile, false, true, merge.NewEnVarFromSlice(os.Environ())),
				Runtime:     fx.Runtime,
			})
		}
	}
	// calculates the package digest
	// the digest is used to check package integrity
	_, digest := s.Checksum(b.workDirZipFilename())
	core.Debug("the package digest is '%s'\n", digest)
	// writes the digest to the seal
	s.Digest = digest
	// save the seal
	core.CheckErr(ioutil.WriteFile(b.workDirJsonFilename(), core.ToJsonBytes(s), os.ModePerm), "failed to write package seal file")
	return s, nil
}

func (b *Builder) sourceDir(workingDirectory string) string {
	return filepath.Join(workingDirectory, core.AppName)
}

// the fully qualified name of the json Seal in the working directory
func (b *Builder) workDirJsonFilename() string {
	return filepath.Join(b.workingDir, fmt.Sprintf("%s.json", b.uniqueIdName))
}

// the fully qualified name of the zip file in the working directory
func (b *Builder) workDirZipFilename() string {
	return filepath.Join(b.workingDir, fmt.Sprintf("%s.zip", b.uniqueIdName))
}

// determine if the from location is a file system path
func (b *Builder) copySource(from string, profile *data.Profile) bool {
	// location is in the file system and no target is specified for the profile
	// should only run commands where the source is
	return !(!strings.HasPrefix(from, "http") && len(profile.Target) == 0)
}

// prepares build specific environment variables
func (b *Builder) getBuildEnv() map[string]string {
	var env = make(map[string]string)
	env["ARTISAN_REF"] = b.uniqueIdName
	env["ARTISAN_BUILD_PATH"] = b.loadFrom
	env["ARTISAN_GIT_COMMIT"] = b.commit
	env["ARTISAN_WORK_DIR"] = b.workingDir
	env["ARTISAN_FROM_URI"] = b.from
	return env
}

// Execute an exported function in a package
func (b *Builder) Execute(name *core.PackageName, function string, credentials string, certPath string, ignoreSignature bool, interactive bool, path string, preserveFiles bool, env *merge.Envar, v registry.Verifier) {
	// get a local registry handle
	local := registry.NewLocalRegistry()
	// check the run path exist
	core.RunPathExists()
	// if no path is specified
	if len(path) == 0 {
		// create a temp random path to open the package
		path = filepath.Join(core.RunPath(), core.RandomString(10))
	} else {
		// otherwise make sure the path is absolute
		path = core.ToAbs(path)
	}
	// open the package on the temp random path
	local.Open(
		name,
		credentials,
		path,
		certPath,
		ignoreSignature,
		v)
	a := local.FindPackage(name)
	// get the package seal
	seal, err := local.GetSeal(a)
	core.CheckErr(err, "cannot get package seal")
	m := seal.Manifest
	// stop execution if the package was built in an OS different from the executing OS
	if runtime.GOOS == "windows" && m.OS != "windows" {
		core.RaiseErr("cannot run package, as it was built in '%s' OS and it is trying to execute in '%s' OS\n"+
			"ensure the package is built under the executing OS\n", m.OS, runtime.GOOS)
	}
	// check the function is exported
	if isExported(m, function) {
		// run the function on the open package
		b.Run(function, path, interactive, env)
		// if there is no instruction to preserve the open files
		if !preserveFiles {
			// remove the package files
			os.RemoveAll(path)
		}
	} else {
		core.RaiseErr("the function '%s' is not defined in the package manifest, check that it has been exported in the build profile\n", function)
	}
}

type Signer interface {
	Sign(data []byte) ([]byte, error)
}
