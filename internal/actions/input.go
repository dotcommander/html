package actions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dotcommander/html/internal/report"
)

func readInput(opts Options) (src []byte, fallbackTitle, sourceName string, err error) {
	if opts.Stdin != nil {
		src, err = readCapped(opts.Stdin, "stdin")
		if err != nil {
			return nil, "", "", err
		}
		if len(src) == 0 {
			return nil, "", "", errors.New("no input on stdin")
		}
		return src, stdinTitle(opts.Title), "", nil
	}
	info, err := os.Stat(opts.File)
	if err != nil {
		return nil, "", "", fmt.Errorf("source file: %w", err)
	}
	if info.IsDir() {
		return nil, "", "", fmt.Errorf("source file: %s is a directory", opts.File)
	}
	f, err := os.Open(opts.File)
	if err != nil {
		return nil, "", "", fmt.Errorf("source file: %w", err)
	}
	src, err = readCapped(f, "source file "+opts.File)
	f.Close()
	if err != nil {
		return nil, "", "", err
	}
	return src, strings.TrimSuffix(filepath.Base(opts.File), filepath.Ext(opts.File)), filepath.Base(opts.File), nil
}

func rejectOutputAlias(opts Options) error {
	if opts.Output == "" || opts.Output == "-" || opts.Stdout {
		return nil
	}
	outputPath, err := filepath.Abs(opts.Output)
	if err != nil {
		return fmt.Errorf("output path: %w", err)
	}
	if opts.Stdin == nil && opts.File != "" {
		if err := rejectPathAlias(outputPath, opts.File, nil, "source file"); err != nil {
			return err
		}
	}
	if opts.templatePath != "" {
		if err := rejectPathAlias(outputPath, opts.templatePath, opts.templateInfo, "template file"); err != nil {
			return err
		}
	}
	return nil
}

func rejectPathAlias(outputPath, sourcePath string, sourceInfo os.FileInfo, label string) error {
	sourcePathAbs, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("%s path: %w", label, err)
	}
	if filepath.Clean(sourcePathAbs) == filepath.Clean(outputPath) {
		return fmt.Errorf("output path aliases %s", label)
	}

	if sourceInfo == nil {
		sourceInfo, err = os.Stat(sourcePath)
		if err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
	}
	outputInfo, err := os.Stat(outputPath)
	if err == nil {
		if os.SameFile(sourceInfo, outputInfo) {
			return fmt.Errorf("output path aliases %s", label)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("output path: %w", err)
	}
	return nil
}

func reportCacheTag(analysis report.Analysis, plan report.ReportPlan, opts Options) string {
	components := make([]reportCacheComponent, 0, len(plan.Components))
	for _, c := range plan.Components {
		components = append(components, reportCacheComponent{Type: c.Type, Title: c.Title})
	}
	renderPlan := struct {
		Analysis   reportCacheAnalysis    `json:"analysis"`
		Layout     report.Layout          `json:"layout"`
		Components []reportCacheComponent `json:"components"`
	}{
		Analysis:   reportCacheAnalysis{Kind: analysis.Kind, Confidence: analysis.Confidence, Reasons: analysis.Reasons, Stats: analysis.Stats},
		Layout:     plan.Layout,
		Components: components,
	}
	b, _ := json.Marshal(renderPlan)
	h := sha256.New()
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

type reportCacheComponent struct {
	Type  report.ComponentType `json:"type"`
	Title string               `json:"title"`
}

type reportCacheAnalysis struct {
	Kind       report.Kind  `json:"kind"`
	Confidence float64      `json:"confidence"`
	Reasons    []string     `json:"reasons"`
	Stats      report.Stats `json:"stats"`
}
