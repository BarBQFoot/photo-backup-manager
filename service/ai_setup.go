package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type AISetupStatus struct {
	Status    string   `json:"status"`
	Phase     string   `json:"phase"`
	Detail    string   `json:"detail"`
	Completed int64    `json:"completed"`
	Total     int64    `json:"total"`
	Unit      string   `json:"unit"`
	Device    string   `json:"device"`
	Error     string   `json:"error"`
	Skipped   []string `json:"skipped"`
}

type AISetup struct {
	mu     sync.Mutex
	status AISetupStatus
	cancel context.CancelFunc
	run    func(context.Context, func(AISetupStatus)) error
}

func NewAISetup() *AISetup {
	return &AISetup{status: AISetupStatus{Status: "idle", Phase: "checking"}, run: setupLocalAI}
}

func (s *AISetup) GetStatus() AISetupStatus { s.mu.Lock(); defer s.mu.Unlock(); return s.status }

func (s *AISetup) Start(parent context.Context) AISetupStatus {
	s.mu.Lock()
	if s.status.Status == "running" || s.status.Status == "ready" {
		status := s.status
		s.mu.Unlock()
		return status
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Hour)
	s.cancel = cancel
	s.status = AISetupStatus{Status: "running", Phase: "checking", Detail: "กำลังตรวจโปรแกรมและโมเดลในเครื่อง"}
	status := s.status
	s.mu.Unlock()
	go func() {
		defer cancel()
		err := s.run(ctx, func(update AISetupStatus) {
			s.mu.Lock()
			defer s.mu.Unlock()
			if update.Device == "" {
				update.Device = s.status.Device
			}
			if update.Skipped == nil {
				update.Skipped = s.status.Skipped
			}
			update.Status = "running"
			s.status = update
		})
		s.mu.Lock()
		defer s.mu.Unlock()
		if err != nil {
			s.status.Status = "error"
			s.status.Error = err.Error()
			if ctx.Err() != nil {
				s.status.Error = "การเตรียมระบบถูกหยุด กรุณาเปิดแอปแล้วลองอีกครั้ง"
			}
			return
		}
		s.status.Status = "ready"
		s.status.Phase = "ready"
		s.status.Detail = "Python และ Florence พร้อมใช้งาน"
		s.status.Completed, s.status.Total, s.status.Unit = 0, 0, ""
	}()
	return status
}

func (s *AISetup) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
}

func managedAIPython(cache string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(cache, "python-env", "Scripts", "python.exe")
	}
	return filepath.Join(cache, "python-env", "bin", "python")
}

// Setup subprocesses do not receive the Gemini credential. uv keeps Python and caches app-local.
func setupEnvironment(cache string) []string {
	env := []string{}
	for _, entry := range aiWorkerEnvironment(os.Environ()) {
		name, _, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(name)
		if strings.Contains(upper, "GEMINI") || upper == "GOOGLE_API_KEY" || strings.HasPrefix(upper, "UV_") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, "UV_PYTHON_INSTALL_DIR="+filepath.Join(cache, "python"),
		"UV_CACHE_DIR="+filepath.Join(cache, "packages"), "UV_NO_CONFIG=1", "UV_NO_PROGRESS=1",
		"UV_HTTP_TIMEOUT=300", "UV_HTTP_CONNECT_TIMEOUT=60", "UV_HTTP_RETRIES=3", "UV_CONCURRENT_DOWNLOADS=2",
		"HF_HUB_DISABLE_TELEMETRY=1", "HF_HUB_DISABLE_SYMLINKS_WARNING=1")
}

type setupReport struct {
	PythonReady       bool   `json:"python_ready"`
	DependenciesReady bool   `json:"dependencies_ready"`
	TorchReady        bool   `json:"torch_ready"`
	TorchVersion      string `json:"torch_version"`
	ModelReady        bool   `json:"model_ready"`
	Signature         string `json:"signature"`
	Device            string `json:"device"`
	Verified          bool   `json:"-"`
	ModelPath         string `json:"model_path"`
	ModelError        string `json:"model_error"`
}

type setupOutput struct {
	mu            sync.Mutex
	buffer        string
	phase         string
	report        setupReport
	lastError     string
	recent        []string
	protocolError bool
	update        func(AISetupStatus)
}

func (w *setupOutput) line(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	var event struct {
		Type string `json:"type"`
		AISetupStatus
		DependenciesReady bool   `json:"dependencies_ready"`
		PythonReady       bool   `json:"python_ready"`
		TorchReady        bool   `json:"torch_ready"`
		TorchVersion      string `json:"torch_version"`
		ModelReady        bool   `json:"model_ready"`
		Signature         string `json:"signature"`
		ModelPath         string `json:"model_path"`
		ModelError        string `json:"model_error"`
	}
	if json.Unmarshal([]byte(line), &event) == nil && event.Type != "" {
		switch event.Type {
		case "check":
			w.report = setupReport{PythonReady: event.PythonReady, DependenciesReady: event.DependenciesReady, TorchReady: event.TorchReady, TorchVersion: event.TorchVersion, ModelReady: event.ModelReady, Signature: event.Signature, ModelPath: event.ModelPath, ModelError: event.ModelError}
		case "verified":
			w.report.Device, w.report.Verified = event.Device, true
		case "error":
			w.lastError = event.Detail
			w.protocolError = true
		case "progress":
			update := event.AISetupStatus
			update.Phase = w.phase
			w.update(update)
		}
		return
	}
	if len(line) > 500 {
		line = line[:500]
	}
	if w.phase == "python" || w.phase == "pytorch" || w.phase == "packages" {
		w.lastError = line
		w.recent = append(w.recent, line)
		if len(w.recent) > 8 {
			w.recent = w.recent[len(w.recent)-8:]
		}
		w.update(AISetupStatus{Phase: w.phase, Detail: line})
	}
}

func (w *setupOutput) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buffer += strings.ReplaceAll(string(data), "\r", "\n")
	for {
		line, remaining, ok := strings.Cut(w.buffer, "\n")
		if !ok {
			break
		}
		w.buffer = remaining
		w.line(line)
	}
	if len(w.buffer) > 16*1024 {
		w.buffer = w.buffer[len(w.buffer)-4096:]
	}
	return len(data), nil
}

func runSetupProcess(ctx context.Context, cache, root, phase, executable string, args []string, update func(AISetupStatus)) (setupReport, error) {
	output := &setupOutput{phase: phase, update: update}
	command := exec.CommandContext(ctx, executable, args...)
	configureAICommand(command)
	command.Dir = root
	command.Env = setupEnvironment(cache)
	command.Stdout, command.Stderr = output, output
	err := command.Run()
	output.mu.Lock()
	output.line(output.buffer)
	output.mu.Unlock()
	if err != nil {
		detail := output.lastError
		if !output.protocolError && len(output.recent) > 0 {
			detail = strings.Join(output.recent, "\n")
		}
		if detail == "" {
			detail = err.Error()
		}
		return output.report, fmt.Errorf("%s: %s", phase, detail)
	}
	return output.report, nil
}

func setupLocalAI(ctx context.Context, update func(AISetupStatus)) error {
	cache, err := aiCacheDir()
	if err != nil {
		return err
	}
	root, err := aiWorkerResources(cache)
	if err != nil {
		return err
	}
	return setupLocalAIWith(ctx, cache, root, aiSetupOperations{discover: discoverSetupPython, uv: prepareUV, command: runSetupProcess, torchIndex: setupTorchIndex}, update)
}

type aiSetupOperations struct {
	discover   func(context.Context, string, string, string, func(AISetupStatus)) (string, setupReport)
	uv         func(context.Context, string, func(AISetupStatus)) (string, error)
	command    func(context.Context, string, string, string, string, []string, func(AISetupStatus)) (setupReport, error)
	torchIndex func(context.Context) string
}

func setupLocalAIWith(ctx context.Context, cache, root string, operations aiSetupOperations, update func(AISetupStatus)) error {
	script := filepath.Join(root, "tools", "bootstrap_ai.py")
	python, report := operations.discover(ctx, cache, root, script, update)
	index := ""
	if operations.torchIndex != nil {
		index = operations.torchIndex(ctx)
	}
	// Installed packages alone do not mean that their CUDA kernels support this GPU.
	repairTorch := strings.HasSuffix(index, "/cu130") && report.TorchReady && !blackwellTorchSupported(report.TorchVersion)
	if repairTorch {
		report.TorchReady = false
		update(AISetupStatus{Phase: "pytorch", Detail: "พบการ์ดจอรุ่นใหม่ กำลังเปลี่ยน PyTorch ให้รองรับ CUDA 13.0"})
	}
	skipped := []string{}
	if python != "" {
		skipped = append(skipped, "python")
	}
	if report.TorchReady {
		skipped = append(skipped, "pytorch")
	}
	if report.DependenciesReady {
		skipped = append(skipped, "bootstrap", "packages")
	}
	if report.ModelReady {
		skipped = append(skipped, "model")
	}
	update(AISetupStatus{Phase: "checking", Detail: "ใช้ส่วนที่มีอยู่แล้ว เตรียมเฉพาะส่วนที่ยังขาด", Skipped: skipped})
	if python == "" && strings.TrimSpace(os.Getenv("PHOTO_AI_PYTHON")) != "" {
		return errors.New("PHOTO_AI_PYTHON ไม่ใช่ Python 3.11–3.13 ที่ใช้งานได้ กรุณาแก้หรือลบค่านี้ก่อนลองใหม่")
	}
	if !report.DependenciesReady || !report.TorchReady {
		uv, err := operations.uv(ctx, cache, update)
		if err != nil {
			return err
		}
		if python == "" || !isVirtualPython(python) {
			base := python
			python = managedAIPython(cache)
			update(AISetupStatus{Phase: "python", Detail: "กำลังดาวน์โหลด Python 3.13 และสร้างสภาพแวดล้อมของแอป"})
			args := []string{"venv", "--python", "3.13", "--managed-python", filepath.Join(cache, "python-env")}
			if base != "" {
				update(AISetupStatus{Phase: "python", Detail: "ใช้ Python ที่ติดตั้งไว้แล้ว กำลังสร้างสภาพแวดล้อมของแอป"})
				args = []string{"venv", "--python", base, "--system-site-packages", filepath.Join(cache, "python-env")}
			}
			if _, err = operations.command(ctx, cache, root, "python", uv,
				args, update); err != nil {
				return err
			}
		}
		if !report.TorchReady {
			detail := "กำลังดาวน์โหลดและติดตั้ง PyTorch"
			if strings.HasSuffix(index, "/cu130") {
				detail = "กำลังติดตั้ง PyTorch CUDA 13.0 สำหรับการ์ดจอรุ่นใหม่"
			}
			update(AISetupStatus{Phase: "pytorch", Detail: detail})
			torchArgs := []string{"pip", "install", "--python", python, "torch", "torchvision"}
			if index != "" {
				torchArgs = []string{"pip", "install", "--python", python, "torch==2.13.0", "torchvision==0.28.0"}
				torchArgs = append(torchArgs, "--default-index", index)
			}
			if repairTorch {
				torchArgs = append(torchArgs, "--reinstall-package", "torch", "--reinstall-package", "torchvision")
			}
			if _, err = operations.command(ctx, cache, root, "pytorch", uv, torchArgs, update); err != nil {
				return err
			}
		}
		if !report.DependenciesReady {
			update(AISetupStatus{Phase: "packages", Detail: "กำลังติดตั้งไลบรารีสำหรับ Florence"})
			if _, err = operations.command(ctx, cache, root, "packages", uv,
				[]string{"pip", "install", "--python", python, "--default-index", "https://pypi.org/simple",
					"transformers>=5,<6", "Pillow", "safetensors", "huggingface_hub"}, update); err != nil {
				return err
			}
		}
		report, err = operations.command(ctx, cache, root, "checking", python, []string{"-u", script, "check"}, update)
		if err != nil || !report.DependenciesReady || !report.TorchReady {
			return errors.New("ติดตั้งไลบรารียังไม่ครบ กรุณาตรวจอินเทอร์เน็ตและพื้นที่ว่างแล้วลองใหม่")
		}
		if strings.HasSuffix(index, "/cu130") && !blackwellTorchSupported(report.TorchVersion) {
			return errors.New("PyTorch ยังไม่รองรับการ์ดจอรุ่นนี้หลังติดตั้ง CUDA 13.0 กรุณาลองใหม่และส่งข้อความผิดพลาด")
		}
	}
	if !report.ModelReady {
		update(AISetupStatus{Phase: "model", Detail: "กำลังดาวน์โหลด Florence-2"})
		if _, err := operations.command(ctx, cache, root, "model", python, []string{"-u", script, "download"}, update); err != nil {
			return err
		}
		var err error
		report, err = operations.command(ctx, cache, root, "checking", python, []string{"-u", script, "check"}, update)
		if err != nil {
			return fmt.Errorf("ตรวจไฟล์ Florence หลังดาวน์โหลดไม่สำเร็จ: %w", err)
		}
		if !report.ModelReady {
			return fmt.Errorf("ตรวจไฟล์ Florence หลังดาวน์โหลดไม่ผ่าน\n%s\nตำแหน่งแคช: %s", report.ModelError, report.ModelPath)
		}
	}
	// Recheck when the selected CUDA runtime changes, as well as Python, packages or model files.
	if report.Signature != "" {
		report.Signature += ":" + index
	}
	hash := sha256.Sum256([]byte(python))
	marker := filepath.Join(cache, "verified-"+hex.EncodeToString(hash[:])[:20]+".json")
	var saved setupReport
	if data, err := os.ReadFile(marker); err == nil {
		_ = json.Unmarshal(data, &saved)
	}
	device := saved.Device
	if report.Signature == "" || saved.Signature != report.Signature {
		update(AISetupStatus{Phase: "verify", Detail: "กำลังตรวจว่าโมเดลอ่านภาพได้จริง"})
		verified, err := operations.command(ctx, cache, root, "verify", python, []string{"-u", script, "verify"}, update)
		if err != nil {
			return err
		}
		if !verified.Verified {
			return errors.New("โมเดลยังไม่ยืนยันว่าทำงานได้ กรุณาลองใหม่")
		}
		device = verified.Device
		report.Device = device
		if err = writeSetupMarker(marker, report); err != nil {
			return err
		}
	} else {
		skipped = append(skipped, "verify")
	}
	if python == managedAIPython(cache) {
		if err := writeSetupMarker(filepath.Join(cache, "python-env-ready.json"), report); err != nil {
			return err
		}
	} else {
		if err := writeSetupMarker(filepath.Join(cache, "existing-python.json"), existingPythonRecord{Python: python}); err != nil {
			return err
		}
	}
	update(AISetupStatus{Phase: "ready", Device: device, Detail: "พร้อมใช้งาน", Skipped: skipped})
	return nil
}

func isVirtualPython(python string) bool {
	_, err := os.Stat(filepath.Join(filepath.Dir(filepath.Dir(python)), "pyvenv.cfg"))
	return err == nil
}

// Keep a validated system interpreter discoverable by the analysis worker after restart.
type existingPythonRecord struct {
	Python string `json:"python"`
}

func discoverSetupPython(ctx context.Context, cache, root, script string, update func(AISetupStatus)) (string, setupReport) {
	candidates := []string{}
	if python, err := aiPython(); err == nil {
		candidates = append(candidates, python)
	}
	if strings.TrimSpace(os.Getenv("PHOTO_AI_PYTHON")) == "" {
		candidates = append(candidates, managedAIPython(cache))
		for _, name := range []string{"python3.13", "python3.12", "python3.11", "python3", "python"} {
			if python, err := exec.LookPath(name); err == nil {
				candidates = append(candidates, python)
			}
		}
		if runtime.GOOS == "darwin" {
			candidates = append(candidates, macSystemPythonPaths()...)
		}
	}
	seen := map[string]bool{}
	var fallback string
	var fallbackReport setupReport
	for _, python := range candidates {
		identity := strings.ToLower(python)
		if seen[identity] || strings.Contains(identity, "windowsapps") {
			continue
		}
		seen[identity] = true
		probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		report, err := runSetupProcess(probeCtx, cache, root, "checking", python, []string{"-u", script, "check"}, update)
		cancel()
		if err != nil || !report.PythonReady {
			continue
		}
		if report.DependenciesReady {
			return python, report
		}
		if fallback == "" {
			fallback, fallbackReport = python, report
		}
	}
	return fallback, fallbackReport
}

// Finder does not normally inherit Homebrew/python.org PATH entries from Terminal.
func macSystemPythonPaths() []string {
	paths := []string{}
	for _, version := range []string{"3.13", "3.12", "3.11"} {
		for _, prefix := range []string{"/opt/homebrew", "/usr/local"} {
			paths = append(paths, prefix+"/bin/python"+version,
				prefix+"/opt/python@"+version+"/bin/python"+version)
		}
		paths = append(paths, "/Library/Frameworks/Python.framework/Versions/"+version+"/bin/python"+version)
	}
	return append(paths, "/opt/homebrew/bin/python3", "/usr/local/bin/python3")
}

func writeSetupMarker(path string, report any) error {
	data, err := json.Marshal(report)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".setup-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, err = file.Write(data)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func setupTorchIndex(ctx context.Context) string {
	if runtime.GOOS != "windows" {
		return ""
	}
	index := "https://download.pytorch.org/whl/cpu"
	if runtime.GOARCH != "amd64" {
		return index
	}
	probe, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := exec.CommandContext(probe, "nvidia-smi", "--query-gpu=name,driver_version,compute_cap", "--format=csv,noheader")
	configureAICommand(command)
	output, err := command.Output()
	if err == nil {
		return torchIndexForNVIDIA(string(output))
	}
	// Older drivers may not expose compute_cap; the name still identifies RTX 50.
	command = exec.CommandContext(probe, "nvidia-smi", "--query-gpu=name,driver_version", "--format=csv,noheader")
	configureAICommand(command)
	if output, err = command.Output(); err == nil {
		return torchIndexForNVIDIA(string(output))
	}
	return index
}

func torchIndexForNVIDIA(output string) string {
	index := "https://download.pytorch.org/whl/cpu"
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(line, ",")
		if len(fields) < 2 {
			continue
		}
		var driver, driverMinor int
		fmt.Sscanf(strings.TrimSpace(fields[1]), "%d.%d", &driver, &driverMinor)
		var capability float64
		if len(fields) > 2 {
			fmt.Sscanf(strings.TrimSpace(fields[2]), "%f", &capability)
		}
		if capability >= 10 || strings.Contains(strings.ToUpper(fields[0]), "RTX 50") {
			// CUDA 13.0 requires an R580 or newer driver. Verify performs a real model run.
			if driver > 580 || driver == 580 && driverMinor >= 88 {
				return "https://download.pytorch.org/whl/cu130"
			}
			return "https://download.pytorch.org/whl/cpu"
		}
		if driver >= 560 {
			index = "https://download.pytorch.org/whl/cu126"
		}
	}
	return index
}

func blackwellTorchSupported(version string) bool {
	var major, minor, cuda int
	fmt.Sscanf(version, "%d.%d", &major, &minor)
	_, suffix, _ := strings.Cut(version, "+cu")
	fmt.Sscanf(suffix, "%d", &cuda)
	return (major > 2 || major == 2 && minor >= 7) && cuda >= 128
}
