package pkg

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"dario.cat/mergo"
	"github.com/charmbracelet/log"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"

	"github.com/mymmrac/tlint/pkg/config"
)

func Run(ctx *cli.Context) error {
	cfgPath := ctx.Path("config")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	log.Infof("Using config: %s", cfgPath)

	// //// //// //

	if cfg.TLint.Dir == "" {
		cfg.TLint.Dir = "./.tlint.yaml"
	}
	if err = os.Mkdir(cfg.TLint.Dir, os.ModePerm); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("internal dir: %w", err)
	}

	// //// //// //

	var configData io.ReadCloser
	switch {
	case cfg.Config.File != "":
		configData, err = os.Open(cfg.Config.File)
		if err != nil {
			return fmt.Errorf("golangci-lint config file: %w", err)
		}

		log.Debugf("Using golangci-lint config file: %s", cfg.Config.File)
	case cfg.Config.URL != "":
		var resp *http.Response
		resp, err = http.Get(cfg.Config.URL)
		if err != nil {
			return fmt.Errorf("golangci-lint config request: %w", err)
		}
		configData = resp.Body

		log.Debugf("Using golangci-lint config file from URL: %s", cfg.Config.URL)
	default:
		return fmt.Errorf("no golangci-lint config found")
	}

	// //// //// //

	configValue := map[string]any{}
	if err = yaml.NewDecoder(configData).Decode(&configValue); err != nil {
		return fmt.Errorf("decode config: %w", err)
	}
	if err = configData.Close(); err != nil {
		log.Debugf("failed to close config reader: %s", err)
	}

	if err = mergo.Merge(&configValue, cfg.Override, mergo.WithOverride); err != nil {
		return fmt.Errorf("override config: %w", err)
	}

	// //// //// //

	configPath := filepath.Join(cfg.TLint.Dir, "golangci-lint.yaml")

	var configFile *os.File
	configFile, err = os.Create(configPath)
	if err != nil {
		return fmt.Errorf("create config: %w", err)
	}

	if err = yaml.NewEncoder(configFile).Encode(configValue); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	// //// //// //

	cmdName := ""
	switch {
	case cfg.GolangCILint.Local:
		cmdName = "golangci-lint"
		log.Debugf("Using local cmd: %q", cmdName)
	case cfg.GolangCILint.File != "":
		cmdName = cfg.GolangCILint.File
		log.Debugf("Using local binary: %q", cmdName)
	case cfg.GolangCILint.URL != "":
		var resp *http.Response
		resp, err = http.Get(cfg.Config.URL)
		if err != nil {
			return fmt.Errorf("golangci-lint binary request: %w", err)
		}

		var cmdData []byte
		cmdData, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("binary read body: %w", err)
		}

		cmdName = filepath.Join(cfg.TLint.Dir, "bin", "golangci-lint")
		if err = os.WriteFile(cmdName, cmdData, 0777); err != nil {
			return fmt.Errorf("create golangci-lint binary: %w", err)
		}
		log.Debugf("Using binary from: %q", cfg.GolangCILint.URL)
	default:
		cmdName = "golangci-lint"
		log.Debugf("Using local cmd (fallback): %q", cmdName)
	}

	// //// //// //

	versionOutput, err := exec.CommandContext(ctx.Context, cmdName, "version").Output()
	if err != nil {
		return fmt.Errorf("golangci-lint version command failed: %w", err)
	}
	versionMatch := regexp.MustCompile(`version ([\w.+]+) built`).FindSubmatch(versionOutput)
	if len(versionMatch) != 2 {
		return fmt.Errorf("golangci-lint version was not found in: %q", string(versionOutput))
	}
	version := string(versionMatch[1])
	log.Infof("Using golangci-lint %s", version)

	// //// //// //

	cmd := exec.CommandContext(ctx.Context, cmdName, "run", "--config", configPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err = cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("golangci-lint run command failed: %w", err)
	}

	return nil
}
