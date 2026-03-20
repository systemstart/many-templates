package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/joho/godotenv"
	"github.com/systemstart/many-templates/pkg/api"
	"github.com/systemstart/many-templates/pkg/logging"
	"github.com/systemstart/many-templates/pkg/processing"
	"github.com/systemstart/many-templates/pkg/resolve"
)

var version = func() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}()

const (
	_ = iota
	exitNoProcessingParameter
	exitDotenvError
	exitLoadConfigurationFileFailed
	exitToolErrors
	exitInputDirectoryNotSpecified
	exitInputDirectoryCheckFailed
	exitInputDirectoryNotADirectory
	exitOutputDirectoryNotSpecified
	exitOutputDirectoryCheckFailed
	exitOutputDirectoryCleanFailed
	exitOutputDirectoryCreateFailed
	exitLoadContextFailed
	exitLoadInstancesFailed
	exitInstanceInputNotADirectory
	exitInstancesIncompatibleFlags
)

var (
	processingFile           string
	instancesFile            string
	inputDirectory           string
	outputDirectory          string
	overwriteOutputDirectory bool
	contextFile              string
	maxDepth                 int
	loggingType              string
	logLevel                 string
	showVersion              bool
	envFile                  string
	noSHA256Update           bool
)

func init() {
	flag.StringVar(
		&processingFile,
		"processing",
		"",
		"single .many.yaml to run (non-recursive mode)")
	flag.StringVar(
		&instancesFile,
		"instances",
		"",
		"instances YAML file for matrix mode")
	flag.StringVar(
		&inputDirectory,
		"input",
		"",
		"input directory (or URI)")
	flag.StringVar(
		&inputDirectory,
		"input-directory",
		"",
		"input directory (alias for -input)")
	flag.StringVar(
		&outputDirectory,
		"output-directory",
		"",
		"output directory")
	flag.BoolVar(
		&overwriteOutputDirectory,
		"overwrite-output-directory",
		false,
		"delete and recreate output directory")
	flag.StringVar(
		&contextFile,
		"context-file",
		"",
		"global context YAML file")
	flag.IntVar(
		&maxDepth,
		"max-depth",
		-1,
		"max directory recursion depth (-1 = unlimited, 0 = root only)")
	flag.StringVar(
		&loggingType,
		"logging-type",
		"tint",
		"logging type: json, text or tint")
	flag.StringVar(
		&logLevel,
		"log-level",
		"info",
		"logging level: debug, info, warn, error")
	flag.BoolVar(
		&showVersion,
		"version",
		false,
		"print version and exit")
	flag.StringVar(
		&envFile,
		"env-file",
		"",
		"load environment variables from file")
	flag.BoolVar(
		&noSHA256Update,
		"no-sha256-update",
		false,
		"disable sha256 writeback to .many.yaml files")
}

func runPull(args []string) {
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: many pull <ref> <dir>\n")
		os.Exit(1)
	}
	ref, dir := args[0], args[1]

	resolved, cleanup, _, err := resolve.Resolve(ref, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if cleanup != nil {
		defer cleanup()
	}

	if err := processing.CopyTree(resolved, dir); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "pull" {
		runPull(os.Args[2:])
		return
	}

	flag.Parse()

	if showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	_ = logging.Initialize(loggingType, logLevel)

	includeEnv()
	cleanup := checkInputDirectory()
	ctxCleanup := resolveContextFile()
	instCleanup := resolveInstancesFile()
	defer func() {
		for _, fn := range []func(){cleanup, ctxCleanup, instCleanup} {
			if fn != nil {
				fn()
			}
		}
	}()
	ensureOutputDirectory()

	if instancesFile != "" && processingFile != "" {
		slog.Error("-instances and -processing are mutually exclusive")
		os.Exit(exitInstancesIncompatibleFlags)
	}

	globalContext := loadGlobalContext()

	if instancesFile != "" {
		runInstancesMode(globalContext)
	} else if processingFile != "" {
		runSinglePipeline(globalContext)
	} else {
		runDiscoveryMode(globalContext)
	}

	slog.Info("done")
}

func runInstancesMode(globalContext map[string]any) {
	cfg, err := api.LoadInstances(instancesFile)
	if err != nil {
		slog.Error("failed to load instances file", "filename", instancesFile, "error", err)
		os.Exit(exitLoadInstancesFailed)
	}

	// Validate instance input directories exist (skip remote URIs — resolved at processing time).
	for _, inst := range cfg.Instances {
		if inst.Input != "" && resolve.IsRemote(inst.Input) {
			continue
		}
		instInputDir := inputDirectory
		if inst.Input != "" {
			instInputDir = filepath.Join(inputDirectory, inst.Input)
		}
		st, err := os.Stat(instInputDir)
		if err != nil || !st.IsDir() {
			slog.Error("instance input is not a directory", "instance", inst.Name, "path", instInputDir)
			os.Exit(exitInstanceInputNotADirectory)
		}
	}

	if err := processing.RunInstances(cfg, inputDirectory, outputDirectory, globalContext, maxDepth, !noSHA256Update); err != nil {
		slog.Error("instances processing failed", "error", err)
		os.Exit(exitToolErrors)
	}
}

func runSinglePipeline(globalContext map[string]any) {
	err := processing.RunSingle(processingFile, inputDirectory, outputDirectory, globalContext, !noSHA256Update)
	if err != nil {
		slog.Error("pipeline failed", "error", err)
		os.Exit(exitToolErrors)
	}
}

func runDiscoveryMode(globalContext map[string]any) {
	err := processing.RunAll(inputDirectory, outputDirectory, globalContext, maxDepth, !noSHA256Update)
	if err != nil {
		slog.Error("processing failed", "error", err)
		os.Exit(exitToolErrors)
	}
}

func loadGlobalContext() map[string]any {
	if contextFile == "" {
		return nil
	}

	ctx, err := processing.LoadContextFile(contextFile)
	if err != nil {
		slog.Error("failed to load context file", "filename", contextFile, "error", err)
		os.Exit(exitLoadContextFailed)
	}
	return ctx
}

func includeEnv() {
	if envFile == "" {
		return
	}
	if err := godotenv.Load(envFile); err != nil {
		slog.Error("failed to load env file", "file", envFile, "error", err)
		os.Exit(exitDotenvError)
	}
	slog.Info("loaded env file", "file", envFile)
}

func checkInputDirectory() func() {
	if inputDirectory == "" {
		slog.Error("-input not set")
		os.Exit(exitInputDirectoryNotSpecified)
	}

	resolved, cleanup, _, err := resolve.Resolve(inputDirectory, "")
	if err != nil {
		slog.Error("failed to resolve input", "input", inputDirectory, "error", err)
		os.Exit(exitInputDirectoryCheckFailed)
	}
	inputDirectory = resolved

	st, err := os.Stat(inputDirectory)
	if err != nil {
		slog.Error("failed to check input directory", "directory", inputDirectory, "error", err)
		os.Exit(exitInputDirectoryCheckFailed)
	}

	if !st.IsDir() {
		slog.Error("-input is not a directory", "directory", inputDirectory)
		os.Exit(exitInputDirectoryNotADirectory)
	}

	return cleanup
}

func resolveContextFile() func() {
	if contextFile == "" {
		return nil
	}
	resolved, cleanup, _, err := resolve.Resolve(contextFile, "")
	if err != nil {
		slog.Error("failed to resolve context file", "file", contextFile, "error", err)
		os.Exit(exitLoadContextFailed)
	}
	contextFile = resolved
	return cleanup
}

func resolveInstancesFile() func() {
	if instancesFile == "" {
		return nil
	}
	resolved, cleanup, _, err := resolve.Resolve(instancesFile, "")
	if err != nil {
		slog.Error("failed to resolve instances file", "file", instancesFile, "error", err)
		os.Exit(exitLoadInstancesFailed)
	}

	// If resolution produced a directory, look for instances.yaml/yml inside it.
	st, stErr := os.Stat(resolved)
	if stErr == nil && st.IsDir() {
		for _, name := range []string{"instances.yaml", "instances.yml"} {
			candidate := filepath.Join(resolved, name)
			if _, cErr := os.Stat(candidate); cErr == nil {
				resolved = candidate
				break
			}
		}
	}

	instancesFile = resolved
	return cleanup
}

func ensureOutputDirectory() {
	if outputDirectory == "" {
		slog.Error("-output-directory not set")
		os.Exit(exitOutputDirectoryNotSpecified)
	}

	_, err := os.Stat(outputDirectory)
	if !os.IsNotExist(err) {
		if err != nil {
			slog.Error("failed to check output directory", "directory", outputDirectory, "error", err)
			os.Exit(exitOutputDirectoryCheckFailed)
		}

		if overwriteOutputDirectory {
			err = os.RemoveAll(outputDirectory)
			if err != nil {
				slog.Error("failed to clean output directory", "directory", outputDirectory, "error", err)
				os.Exit(exitOutputDirectoryCleanFailed)
			}
		}
	}

	err = os.MkdirAll(outputDirectory, 0o750)
	if err != nil {
		slog.Error("failed to create output directory", "directory", outputDirectory, "error", err)
		os.Exit(exitOutputDirectoryCreateFailed)
	}
}
