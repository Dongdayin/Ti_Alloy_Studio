package engines

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"tialloystudio/internal/model"
)

type ATATCorrelation struct {
	Points     int     `json:"points"`
	Diameter   float64 `json:"diameter"`
	Observed   float64 `json:"observed"`
	Target     float64 `json:"target"`
	Difference float64 `json:"difference"`
}

type ATATQuality struct {
	Clusters         []ATATCorrelation `json:"clusters"`
	RMSDifference    float64           `json:"rms_difference"`
	MaxAbsDifference float64           `json:"max_abs_difference"`
}

type ATATOptions struct {
	Distro        string  `json:"distro"`
	TotalSites    int     `json:"total_sites"`
	PairCutoff    float64 `json:"pair_cutoff_angstrom"`
	TripletCutoff float64 `json:"triplet_cutoff_angstrom"`
	RunSeconds    int     `json:"run_seconds"`
}

type ATATRunResult struct {
	Structure  model.Structure `json:"structure"`
	Quality    ATATQuality     `json:"quality"`
	Command    string          `json:"command"`
	Distro     string          `json:"distro,omitempty"`
	ReturnCode int             `json:"return_code"`
	Stdout     string          `json:"stdout"`
	Stderr     string          `json:"stderr"`
	WorkDir    string          `json:"work_dir"`
}

func BuildATATRndStr(host model.Structure, atomicPercent map[string]float64) (string, error) {
	if host.NAtoms() == 0 || host.Volume() <= 0 {
		return "", errors.New("ATAT parent lattice is empty or singular")
	}
	if len(atomicPercent) == 0 {
		return "", errors.New("ATAT composition is empty")
	}
	elements := make([]string, 0, len(atomicPercent))
	total := 0.0
	for e, v := range atomicPercent {
		if _, ok := model.AtomicWeights[e]; !ok {
			return "", fmt.Errorf("unknown element %q", e)
		}
		if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return "", fmt.Errorf("invalid atomic percentage for %s", e)
		}
		total += v
		elements = append(elements, e)
	}
	if math.Abs(total-100) > 1e-7 {
		return "", fmt.Errorf("ATAT atomic percentages must sum to 100; got %.12g", total)
	}
	sort.Strings(elements)

	var b strings.Builder
	for i := 0; i < 3; i++ {
		fmt.Fprintf(&b, "%.12g %.12g %.12g\n", host.Cell[i][0], host.Cell[i][1], host.Cell[i][2])
	}
	b.WriteString("1 0 0\n0 1 0\n0 0 1\n")
	frac := host.Fractional(true)
	for _, f := range frac {
		fmt.Fprintf(&b, "%.12g %.12g %.12g ", f[0], f[1], f[2])
		for j, e := range elements {
			if j > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, "%s=%.10g", e, atomicPercent[e]/100)
		}
		b.WriteByte('\n')
	}
	return b.String(), nil
}

func ParseATATBestCorr(text string) (ATATQuality, error) {
	var out ATATQuality
	s := bufio.NewScanner(strings.NewReader(text))
	sum2 := 0.0
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		p, err1 := strconv.Atoi(fields[0])
		d, err2 := strconv.ParseFloat(fields[1], 64)
		o, err3 := strconv.ParseFloat(fields[2], 64)
		t, err4 := strconv.ParseFloat(fields[3], 64)
		diff, err5 := strconv.ParseFloat(fields[4], 64)
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil {
			continue
		}
		c := ATATCorrelation{Points: p, Diameter: d, Observed: o, Target: t, Difference: diff}
		out.Clusters = append(out.Clusters, c)
		sum2 += diff * diff
		out.MaxAbsDifference = math.Max(out.MaxAbsDifference, math.Abs(diff))
	}
	if err := s.Err(); err != nil {
		return ATATQuality{}, err
	}
	if len(out.Clusters) == 0 {
		return ATATQuality{}, errors.New("bestcorr.out contained no parseable cluster correlations")
	}
	out.RMSDifference = math.Sqrt(sum2 / float64(len(out.Clusters)))
	return out, nil
}

func parseVec3(line string) (model.Vec3, error) {
	f := strings.Fields(strings.TrimSpace(line))
	if len(f) < 3 {
		return model.Vec3{}, errors.New("expected three numeric coordinates")
	}
	var v model.Vec3
	for i := 0; i < 3; i++ {
		x, err := strconv.ParseFloat(f[i], 64)
		if err != nil {
			return model.Vec3{}, err
		}
		v[i] = x
	}
	return v, nil
}

func linearCombination(coeff model.Vec3, basis model.Mat3) model.Vec3 {
	return model.VAdd(model.VAdd(model.VScale(basis[0], coeff[0]), model.VScale(basis[1], coeff[1])), model.VScale(basis[2], coeff[2]))
}

func ParseATATStructure(text string) (model.Structure, error) {
	lines := make([]string, 0)
	s := bufio.NewScanner(strings.NewReader(text))
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}
	if err := s.Err(); err != nil {
		return model.Structure{}, err
	}
	if len(lines) < 7 {
		return model.Structure{}, errors.New("ATAT structure is incomplete")
	}
	var coord model.Mat3
	for i := 0; i < 3; i++ {
		v, err := parseVec3(lines[i])
		if err != nil {
			return model.Structure{}, fmt.Errorf("coordinate basis line %d: %w", i+1, err)
		}
		coord[i] = v
	}
	var cell model.Mat3
	for i := 0; i < 3; i++ {
		c, err := parseVec3(lines[3+i])
		if err != nil {
			return model.Structure{}, fmt.Errorf("lattice-vector line %d: %w", i+1, err)
		}
		cell[i] = linearCombination(c, coord)
	}
	out := model.Structure{Cell: cell, PBC: [3]bool{true, true, true}, Meta: map[string]any{"source": "ATAT bestsqs.out"}}
	for i, line := range lines[6:] {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			return model.Structure{}, fmt.Errorf("ATAT atom line %d is incomplete", i+1)
		}
		coeff, err := parseVec3(line)
		if err != nil {
			return model.Structure{}, fmt.Errorf("ATAT atom line %d: %w", i+1, err)
		}
		sp := strings.Split(fields[3], "=")[0]
		sp = strings.Split(sp, ",")[0]
		sp = strings.TrimSpace(sp)
		if sp == "" {
			return model.Structure{}, fmt.Errorf("ATAT atom line %d has no species", i+1)
		}
		out.Positions = append(out.Positions, linearCombination(coeff, coord))
		out.Species = append(out.Species, sp)
	}
	if out.NAtoms() == 0 || out.Volume() <= 0 {
		return model.Structure{}, errors.New("parsed ATAT structure is empty or singular")
	}
	return out, nil
}

func ParseWSLDistros(raw string) []string {
	raw = strings.ReplaceAll(raw, "\x00", "")
	raw = strings.TrimPrefix(raw, "\ufeff")
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimPrefix(line, "*"))
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func wslPrefix(distro string) []string {
	if strings.TrimSpace(distro) == "" {
		return nil
	}
	return []string{"-d", strings.TrimSpace(distro)}
}

func quoteBash(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func wslPath(wslExe, distro, windowsPath string) (string, error) {
	args := append(wslPrefix(distro), "--", "wslpath", "-a", windowsPath)
	b, err := exec.Command(wslExe, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("wslpath failed: %w: %s", err, strings.TrimSpace(string(b)))
	}
	p := strings.TrimSpace(strings.ReplaceAll(string(b), "\x00", ""))
	if p == "" {
		return "", errors.New("wslpath returned an empty path")
	}
	return p, nil
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

func RunATATSQS(parent model.Structure, atomicPercent map[string]float64, opt ATATOptions) (ATATRunResult, error) {
	if opt.TotalSites < 1 {
		return ATATRunResult{}, errors.New("ATAT total_sites must be >= 1")
	}
	if opt.PairCutoff <= 0 || math.IsNaN(opt.PairCutoff) || math.IsInf(opt.PairCutoff, 0) {
		return ATATRunResult{}, errors.New("ATAT pair cutoff must be explicitly specified and positive")
	}
	if opt.TripletCutoff < 0 || math.IsNaN(opt.TripletCutoff) || math.IsInf(opt.TripletCutoff, 0) {
		return ATATRunResult{}, errors.New("ATAT triplet cutoff must be zero or positive")
	}
	if opt.RunSeconds < 1 {
		return ATATRunResult{}, errors.New("ATAT run_seconds must be >= 1")
	}
	rnd, err := BuildATATRndStr(parent, atomicPercent)
	if err != nil {
		return ATATRunResult{}, err
	}
	wslExe, err := exec.LookPath("wsl.exe")
	if err != nil {
		return ATATRunResult{}, errors.New("wsl.exe was not found; ATAT/mcsqs requires the configured WSL environment")
	}
	work, err := os.MkdirTemp("", "TiAlloyStudio-ATAT-")
	if err != nil {
		return ATATRunResult{}, err
	}
	if err := os.WriteFile(filepath.Join(work, "rndstr.in"), []byte(rnd), 0644); err != nil {
		return ATATRunResult{}, err
	}
	linuxWork, err := wslPath(wslExe, opt.Distro, work)
	if err != nil {
		return ATATRunResult{}, err
	}

	mcsqsArgs := []string{fmt.Sprintf("-n=%d", opt.TotalSites), fmt.Sprintf("-2=%.12g", opt.PairCutoff)}
	if opt.TripletCutoff > 0 {
		mcsqsArgs = append(mcsqsArgs, fmt.Sprintf("-3=%.12g", opt.TripletCutoff))
	}
	mcsqsText := "mcsqs " + strings.Join(mcsqsArgs, " ")
	script := "set -u; cd " + quoteBash(linuxWork) + "; rm -f stopsqs; " + mcsqsText + " >mcsqs.stdout 2>mcsqs.stderr & pid=$!; (sleep " + strconv.Itoa(opt.RunSeconds) + "; if kill -0 $pid 2>/dev/null; then touch stopsqs; fi) & watcher=$!; wait $pid; rc=$?; kill $watcher 2>/dev/null || true; wait $watcher 2>/dev/null || true; exit $rc"
	args := append(wslPrefix(opt.Distro), "--", "bash", "-lc", script)
	cmd := exec.Command(wslExe, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	if b, e := os.ReadFile(filepath.Join(work, "mcsqs.stdout")); e == nil {
		stdout.Write(b)
	}
	if b, e := os.ReadFile(filepath.Join(work, "mcsqs.stderr")); e == nil {
		stderr.Write(b)
	}
	result := ATATRunResult{
		Command:    mcsqsText,
		Distro:     opt.Distro,
		ReturnCode: exitCode(runErr),
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		WorkDir:    work,
	}
	bestSQS, sqsErr := os.ReadFile(filepath.Join(work, "bestsqs.out"))
	bestCorr, corrErr := os.ReadFile(filepath.Join(work, "bestcorr.out"))
	if sqsErr != nil || corrErr != nil {
		if runErr != nil {
			return result, fmt.Errorf("mcsqs failed before producing bestsqs.out/bestcorr.out (exit %d): %s", result.ReturnCode, strings.TrimSpace(result.Stderr))
		}
		return result, errors.New("mcsqs did not produce bestsqs.out and bestcorr.out")
	}
	result.Structure, err = ParseATATStructure(string(bestSQS))
	if err != nil {
		return result, fmt.Errorf("parse bestsqs.out: %w", err)
	}
	result.Quality, err = ParseATATBestCorr(string(bestCorr))
	if err != nil {
		return result, fmt.Errorf("parse bestcorr.out: %w", err)
	}
	result.Structure.Meta["model_kind"] = "sqs"
	result.Structure.Meta["sqs_engine"] = "ATAT mcsqs"
	result.Structure.Meta["atat_pair_cutoff_angstrom"] = opt.PairCutoff
	result.Structure.Meta["atat_triplet_cutoff_angstrom"] = opt.TripletCutoff
	result.Structure.Meta["atat_run_seconds"] = opt.RunSeconds
	return result, nil
}
