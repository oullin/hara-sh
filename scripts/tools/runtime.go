// The tools entrypoint initialises private state and launches containerised helpers.
package main

import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

type toolRuntime struct {
	state    string
	template string
	url      string
	client   *http.Client
	execute  func(string, []string, []string) error
}

const (
	apiField        = "claude-api-key"
	managementField = "management-password"
)

func main() {
	runtime := toolRuntime{
		state:    cmp.Or(os.Getenv("HARA_STATE"), "/state"),
		template: cmp.Or(os.Getenv("HARA_TEMPLATE"), "/template.yaml"),
		url:      cmp.Or(os.Getenv("URL"), "http://proxy:8317"),
		client: &http.Client{
			Timeout:       10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		execute: syscall.Exec,
	}

	if err := runtime.run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func validateKey(value string) error {
	if len(value) < 20 || len(value) > 72 || !utf8.ValidString(value) || strings.ContainsAny(value, "\r\n\x00") {
		return errors.New("credentials must be 20–72 UTF-8 bytes without line breaks")
	}

	return nil
}

func (runtime toolRuntime) readKey(field string) (string, error) {
	if field != apiField && field != managementField {
		return "", errors.New("unknown credential field")
	}

	content, err := os.ReadFile(filepath.Join(runtime.state, "secrets", field))

	if err != nil {
		return "", fmt.Errorf("read credential file: %w", err)
	}

	value := strings.TrimSuffix(string(content), "\n")

	return value, validateKey(value)
}

func (runtime toolRuntime) privateWrite(path, value string) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".hara-*")

	if err != nil {
		return err
	}

	defer os.Remove(temporary.Name())

	defer temporary.Close()

	if _, err := io.WriteString(temporary, value); err != nil {
		return err
	}

	if err := runtime.setOwner(temporary.Name()); err != nil {
		return err
	}

	if err := temporary.Close(); err != nil {
		return err
	}

	return os.Rename(temporary.Name(), path)
}

func (runtime toolRuntime) setOwner(path string) error {
	owner, err := os.Stat(runtime.state)

	if err != nil {
		return err
	}

	stat := owner.Sys().(*syscall.Stat_t)

	return os.Chown(path, int(stat.Uid), int(stat.Gid))
}

func generateKey() (string, error) {
	var value [32]byte

	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}

	return hex.EncodeToString(value[:]), nil
}

func importKeys(input io.Reader) ([]string, error) {
	// Bound input before parsing; two 72-byte keys and two CRLFs need at most 148 bytes.
	content, err := io.ReadAll(io.LimitReader(input, 149))

	if err != nil {
		return nil, errors.New("read imported credentials")
	}

	values := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")

	if len(content) > 148 || len(values) != 2 {
		return nil, errors.New("import requires exactly two credential lines")
	}

	for i := range values {
		values[i] = strings.TrimSuffix(values[i], "\r")
	}

	return values, nil
}

func (runtime toolRuntime) credentials(mode string, input io.Reader) ([]string, error) {
	fields := []string{apiField, managementField}
	present := 0

	for _, field := range fields {
		info, err := os.Stat(filepath.Join(runtime.state, "secrets", field))

		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}

		if err == nil && info.Mode().IsRegular() {
			present++
		}
	}

	if mode == "import" {
		return importKeys(input)
	}

	if mode == "rotate" && present != len(fields) {
		return nil, errors.New("initialise or import credentials before rotating")
	}

	if present == 1 {
		return nil, errors.New("incomplete credential files; restore both from your private backup")
	}

	if present == len(fields) && mode != "rotate" {
		values := make([]string, len(fields))

		for i, field := range fields {
			value, err := runtime.readKey(field)

			if err != nil {
				return nil, err
			}

			values[i] = value
		}

		return values, nil
	}

	if _, err := os.Stat(filepath.Join(runtime.state, "proxy", "config.yaml")); err == nil {
		if mode != "rotate" {
			return nil, errors.New("existing installation: import its two credentials before starting; see the migration guide")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	values := make([]string, len(fields))

	for i := range values {
		value, err := generateKey()

		if err != nil {
			return nil, err
		}

		values[i] = value
	}

	return values, nil
}

func (runtime toolRuntime) initialise(mode string, input io.Reader, output io.Writer) error {
	values, err := runtime.credentials(mode, input)

	if err != nil {
		return err
	}

	for _, value := range values {
		if err := validateKey(value); err != nil {
			return err
		}
	}

	template, err := os.ReadFile(runtime.template)

	if err != nil {
		return err
	}

	config := string(template)

	if strings.Count(config, "__API_KEY__") != 1 || strings.Count(config, "__MANAGEMENT_PASSWORD_BCRYPT__") != 1 {
		return errors.New("configuration template must contain both credential placeholders exactly once")
	}

	if !strings.Contains(config, `"__API_KEY__"`) || !strings.Contains(config, `"__MANAGEMENT_PASSWORD_BCRYPT__"`) {
		return errors.New("credential placeholders must be quoted")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(values[1]), bcrypt.DefaultCost)

	if err != nil {
		return errors.New("hash management password")
	}

	apiJSON, err := json.Marshal(values[0])

	if err != nil {
		return errors.New("encode client key")
	}

	hashJSON, err := json.Marshal(string(hash))

	if err != nil {
		return errors.New("encode management password hash")
	}

	config = strings.Replace(config, `"__API_KEY__"`, string(apiJSON), 1)
	config = strings.Replace(config, `"__MANAGEMENT_PASSWORD_BCRYPT__"`, string(hashJSON), 1)

	if err := os.MkdirAll(runtime.state, 0o700); err != nil {
		return err
	}

	if err := os.Chmod(runtime.state, 0o700); err != nil {
		return err
	}

	for _, name := range []string{"secrets", "proxy", "tailscale"} {
		path := filepath.Join(runtime.state, name)

		if err := os.MkdirAll(path, 0o700); err != nil {
			return err
		}

		if err := runtime.setOwner(path); err != nil {
			return err
		}

		if err := os.Chmod(path, 0o700); err != nil {
			return err
		}
	}

	for i, field := range []string{apiField, managementField} {
		if err := runtime.privateWrite(filepath.Join(runtime.state, "secrets", field), values[i]+"\n"); err != nil {
			return err
		}
	}

	if err := runtime.privateWrite(filepath.Join(runtime.state, "proxy", "config.yaml"), config); err != nil {
		return err
	}

	_, err = fmt.Fprintln(output, "Private configuration ready. Existing credentials and provider logins are preserved unless import or rotate was requested.")

	return err
}

func (runtime toolRuntime) request(path, field string) ([]byte, error) {
	request, err := http.NewRequest(http.MethodGet, strings.TrimRight(runtime.url, "/")+path, nil)

	if err != nil {
		return nil, errors.New("invalid proxy URL")
	}

	if field != "" {
		key, err := runtime.readKey(field)

		if err != nil {
			return nil, err
		}

		request.Header.Set("Authorization", "Bearer "+key)
	}

	response, err := runtime.client.Do(request)

	if err != nil {
		return nil, fmt.Errorf("request %s failed", path)
	}

	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("request %s returned HTTP %d", path, response.StatusCode)
	}

	content, err := io.ReadAll(io.LimitReader(response.Body, 4<<20+1))

	if err != nil || len(content) > 4<<20 {
		return nil, fmt.Errorf("read response from %s failed", path)
	}

	return content, nil
}

func (runtime toolRuntime) status(output io.Writer) error {
	for _, check := range []struct{ path, message string }{
		{"/healthz", "✓ proxy health"},
		{"/management.html", "✓ management panel"},
	} {
		if _, err := runtime.request(check.path, ""); err != nil {
			return err
		}

		fmt.Fprintln(output, check.message)
	}

	content, err := runtime.request("/v1/models", apiField)

	if err != nil {
		return err
	}

	var models struct {
		Data []any `json:"data"`
	}

	if err := json.Unmarshal(content, &models); err != nil {
		return errors.New("invalid model list response")
	}

	fmt.Fprintf(output, "✓ client key accepted, %d models\n", len(models.Data))
	content, err = runtime.request("/v0/management/auth-files", managementField)

	if err != nil {
		return err
	}

	var accounts struct {
		Files []any `json:"files"`
	}

	if err := json.Unmarshal(content, &accounts); err != nil {
		return errors.New("invalid provider account response")
	}

	if len(accounts.Files) == 0 {
		fmt.Fprintln(output, "! no provider accounts; connect one in the management panel")
	} else {
		fmt.Fprintf(output, "✓ management key accepted, %d provider accounts\n", len(accounts.Files))
	}

	return nil
}

func (runtime toolRuntime) launch(command string, args []string) error {
	fields := []string{apiField}

	switch command {
	case "quota":
		fields = []string{managementField}
	case "ops":
		if len(args) == 0 || args[0] == "status" {
			return errors.New("use the status command for containerised health checks")
		}

		if !slices.ContainsFunc(args, func(arg string) bool { return arg == "-url" || strings.HasPrefix(arg, "-url=") }) {
			args = append([]string{args[0], "-url", runtime.url}, args[1:]...)
		}

		fields = []string{apiField, managementField}
	}

	env := slices.DeleteFunc(os.Environ(), func(value string) bool {
		return strings.HasPrefix(value, "API_KEY=") || strings.HasPrefix(value, "MGMT_KEY=")
	})

	for _, field := range fields {
		value, err := runtime.readKey(field)

		if err != nil {
			return err
		}

		name := "API_KEY"

		if field == managementField {
			name = "MGMT_KEY"
		}

		env = append(env, name+"="+value)
	}

	return runtime.execute("/usr/local/bin/"+command, append([]string{command}, args...), env)
}

func (runtime toolRuntime) run(args []string, input io.Reader, output io.Writer) error {
	if len(args) == 0 {
		args = []string{"status"}
	}

	command, rest := args[0], args[1:]

	switch command {
	case "init", "import", "rotate", "status", "health":
		if len(rest) != 0 {
			return errors.New("unexpected tool arguments")
		}

		switch command {
		case "init", "import", "rotate":
			return runtime.initialise(command, input, output)
		case "status":
			return runtime.status(output)
		default:
			_, err := runtime.request("/healthz", "")

			return err
		}
	case "key":
		if len(rest) != 1 {
			return errors.New("usage: key claude-api-key|management-password")
		}

		value, err := runtime.readKey(rest[0])

		if err != nil {
			return err
		}

		_, err = fmt.Fprintln(output, value)

		return err
	case "ops", "quota", "wssmoke", "bench":
		return runtime.launch(command, rest)
	default:
		return errors.New("unknown tool; choose init, import, rotate, key, status, health, ops, quota, wssmoke or bench")
	}
}
