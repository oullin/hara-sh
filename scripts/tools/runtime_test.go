package main

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

const testTemplate = "key: \"__API_KEY__\"\nmanagement: \"__MANAGEMENT_PASSWORD_BCRYPT__\"\n"

func newTestRuntime(t *testing.T) toolRuntime {
	t.Helper()

	root := t.TempDir()
	runtime := toolRuntime{state: filepath.Join(root, "state"), template: filepath.Join(root, "template.yaml"), url: "http://proxy:8317"}

	if err := os.WriteFile(runtime.template, []byte(testTemplate), 0o600); err != nil {
		t.Fatal(err)
	}

	return runtime
}

func initialiseForTest(t *testing.T, runtime toolRuntime, mode, input string) string {
	t.Helper()

	var output bytes.Buffer

	if err := runtime.run([]string{mode}, strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}

	return output.String()
}

func keyForTest(t *testing.T, runtime toolRuntime, field string) string {
	t.Helper()

	value, err := runtime.readKey(field)

	if err != nil {
		t.Fatal(err)
	}

	return value
}

func readForTest(t *testing.T, path string) []byte {
	t.Helper()

	content, err := os.ReadFile(path)

	if err != nil {
		t.Fatal(err)
	}

	return content
}

func TestFreshInstallPreservesCredentialsAndProviderLogins(t *testing.T) {
	runtime := newTestRuntime(t)
	output := initialiseForTest(t, runtime, "init", "")
	api, management := keyForTest(t, runtime, apiField), keyForTest(t, runtime, managementField)

	if api == management || len(api) != 64 || len(management) != 64 {
		t.Fatal("credentials must be distinct 256-bit random hex values")
	}

	if strings.Contains(output, api) || strings.Contains(output, management) {
		t.Fatal("initialisation exposed credentials")
	}

	config := string(readForTest(t, filepath.Join(runtime.state, "proxy", "config.yaml")))

	if !strings.Contains(config, api) || strings.Contains(config, management) {
		t.Fatal("config must contain the client key and only a hash of the management password")
	}

	_, encoded, _ := strings.Cut(config, "management: ")

	var hash string

	if err := json.Unmarshal([]byte(encoded), &hash); err != nil {
		t.Fatal(err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(management)); err != nil {
		t.Fatal("management hash does not authenticate the password")
	}

	if cost, err := bcrypt.Cost([]byte(hash)); err != nil || cost != 10 || !strings.HasPrefix(hash, "$2a$") {
		t.Fatal("management hash must retain the proxy-compatible bcrypt format and cost")
	}

	auths := filepath.Join(runtime.state, "proxy", "auths")

	if err := os.Mkdir(auths, 0o700); err != nil {
		t.Fatal(err)
	}

	account := filepath.Join(auths, "account.json")

	if err := os.WriteFile(account, []byte("private provider state"), 0o600); err != nil {
		t.Fatal(err)
	}

	initialiseForTest(t, runtime, "init", "")

	if keyForTest(t, runtime, apiField) != api || keyForTest(t, runtime, managementField) != management || string(readForTest(t, account)) != "private provider state" {
		t.Fatal("reinitialisation changed credentials or provider logins")
	}

	for _, name := range []string{"", "secrets", "proxy", "tailscale", "secrets/" + apiField, "secrets/" + managementField, "proxy/config.yaml"} {
		info, err := os.Stat(filepath.Join(runtime.state, name))

		if err != nil {
			t.Fatal(err)
		}

		want := os.FileMode(0o600)

		if info.IsDir() {
			want = 0o700
		}

		if info.Mode().Perm() != want {
			t.Errorf("%s permissions = %o; want %o", name, info.Mode().Perm(), want)
		}
	}

	leftovers, err := filepath.Glob(filepath.Join(runtime.state, "*", ".hara-*"))

	if err != nil || len(leftovers) != 0 {
		t.Fatal("atomic writes left temporary credential files")
	}
}

func TestLegacyAndPartialInstallationsAreNotRegenerated(t *testing.T) {
	t.Run("legacy", func(t *testing.T) {
		runtime := newTestRuntime(t)
		config := filepath.Join(runtime.state, "proxy", "config.yaml")

		if err := os.MkdirAll(filepath.Dir(config), 0o700); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(config, []byte("old configuration"), 0o600); err != nil {
			t.Fatal(err)
		}

		err := runtime.run([]string{"init"}, strings.NewReader(""), io.Discard)

		if err == nil || !strings.Contains(err.Error(), "existing installation") || string(readForTest(t, config)) != "old configuration" {
			t.Fatal("legacy installation was not preserved")
		}
	})
	t.Run("partial", func(t *testing.T) {
		runtime := newTestRuntime(t)
		initialiseForTest(t, runtime, "init", "")
		api := keyForTest(t, runtime, apiField)

		if err := os.Remove(filepath.Join(runtime.state, "secrets", managementField)); err != nil {
			t.Fatal(err)
		}

		for _, command := range []string{"init", "rotate"} {
			if err := runtime.run([]string{command}, strings.NewReader(""), io.Discard); err == nil {
				t.Fatal("partial credentials were regenerated")
			}
		}

		if keyForTest(t, runtime, apiField) != api {
			t.Fatal("partial installation lost its existing key")
		}
	})
}

func TestInitialisationPreservesMountedStateOwner(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("ownership changes require the root user used by the Docker image")
	}

	runtime := newTestRuntime(t)

	if err := os.Mkdir(runtime.state, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.Chown(runtime.state, 1001, 1002); err != nil {
		t.Fatal(err)
	}

	initialiseForTest(t, runtime, "init", "")

	for _, path := range []string{"", "secrets", "proxy", "tailscale", "secrets/" + apiField, "secrets/" + managementField, "proxy/config.yaml"} {
		info, err := os.Stat(filepath.Join(runtime.state, path))

		if err != nil {
			t.Fatal(err)
		}

		owner := info.Sys().(*syscall.Stat_t)

		if owner.Uid != 1001 || owner.Gid != 1002 {
			t.Errorf("state ownership changed for %s", path)
		}
	}
}

func TestImportPreservesQuotedCredentialsAndRotationIsExplicit(t *testing.T) {
	runtime := newTestRuntime(t)
	api := " quoted\"key\\with-special-characters "
	management := "separate-long-management-password"
	output := initialiseForTest(t, runtime, "import", api+"\r\n"+management+"\r\n")

	if keyForTest(t, runtime, apiField) != api || keyForTest(t, runtime, managementField) != management {
		t.Fatal("import altered credentials")
	}

	if strings.Contains(output, api) || strings.Contains(output, management) {
		t.Fatal("import exposed credentials")
	}

	config := string(readForTest(t, filepath.Join(runtime.state, "proxy", "config.yaml")))
	line, _, _ := strings.Cut(config, "\n")

	var decoded string

	if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "key: ")), &decoded); err != nil || decoded != api {
		t.Fatal("config did not escape the imported client key")
	}

	initialiseForTest(t, runtime, "rotate", "")

	if keyForTest(t, runtime, apiField) == api || keyForTest(t, runtime, managementField) == management {
		t.Fatal("rotation did not replace both credentials")
	}
}

func TestInvalidInputDoesNotChangeExistingCredentialsOrConfig(t *testing.T) {
	runtime := newTestRuntime(t)
	initialiseForTest(t, runtime, "init", "")
	paths := []string{"secrets/" + apiField, "secrets/" + managementField, "proxy/config.yaml"}
	before := make([][]byte, len(paths))

	for i, path := range paths {
		before[i] = readForTest(t, filepath.Join(runtime.state, path))
	}

	assertUnchanged := func(t *testing.T) {
		t.Helper()

		for i, path := range paths {
			if !bytes.Equal(readForTest(t, filepath.Join(runtime.state, path)), before[i]) {
				t.Errorf("invalid input changed %s", path)
			}
		}
	}

	for _, input := range []string{
		"short\nshort\n", strings.Repeat("a", 73) + "\n" + strings.Repeat("b", 32) + "\n",
		strings.Repeat("a", 32) + "\n" + strings.Repeat("b", 32) + "\nextra",
		strings.Repeat("a", 32) + "\x00\n" + strings.Repeat("b", 32) + "\n",
		strings.Repeat("界", 25) + "\n" + strings.Repeat("b", 32) + "\n",
		strings.Repeat("a", 32) + "\xff\n" + strings.Repeat("b", 32) + "\n",
	} {
		err := runtime.run([]string{"import"}, strings.NewReader(input), io.Discard)

		if err == nil {
			t.Fatal("invalid import was accepted")
		}

		if strings.Contains(err.Error(), input) {
			t.Fatal("invalid import exposed its contents")
		}

		assertUnchanged(t)
	}

	for _, template := range []string{"missing placeholders", testTemplate + "__API_KEY__", strings.ReplaceAll(testTemplate, `"`, "")} {
		if err := os.WriteFile(runtime.template, []byte(template), 0o600); err != nil {
			t.Fatal(err)
		}

		if err := runtime.run([]string{"rotate"}, strings.NewReader(""), io.Discard); err == nil {
			t.Fatal("invalid template was accepted")
		}

		assertUnchanged(t)
	}
}

func TestLaunchLoadsOnlyRequiredCredentialsAndPreservesArguments(t *testing.T) {
	runtime := newTestRuntime(t)
	initialiseForTest(t, runtime, "init", "")
	t.Setenv("API_KEY", "inherited-key-must-not-leak")
	t.Setenv("MGMT_KEY", "inherited-management-must-not-leak")

	for _, tc := range []struct {
		command         string
		args            []string
		want            []string
		api, management bool
	}{
		{"quota", []string{"-route"}, []string{"quota", "-route"}, false, true},
		{"ops", []string{"models"}, []string{"ops", "models", "-url", runtime.url}, true, true},
		{"ops", []string{"models", "-url=http://custom:8317"}, []string{"ops", "models", "-url=http://custom:8317"}, true, true},
		{"ops", []string{"models", "-url", "http://custom:8317"}, []string{"ops", "models", "-url", "http://custom:8317"}, true, true},
		{"wssmoke", []string{"-idle", "2m", runtime.url}, []string{"wssmoke", "-idle", "2m", runtime.url}, true, false},
		{"bench", []string{"-n", "2", runtime.url}, []string{"bench", "-n", "2", runtime.url}, true, false},
	} {
		t.Run(tc.command+strings.Join(tc.args, " "), func(t *testing.T) {
			called := false
			runtime.execute = func(path string, args, env []string) error {
				called = true

				if path != "/usr/local/bin/"+tc.command || !reflect.DeepEqual(args, tc.want) {
					t.Fatal("tool executable or arguments changed")
				}

				for _, field := range []struct {
					name, field string
					required    bool
				}{
					{"API_KEY", apiField, tc.api}, {"MGMT_KEY", managementField, tc.management},
				} {
					found := slices.Contains(env, field.name+"="+keyForTest(t, runtime, field.field))

					if found != field.required {
						t.Errorf("incorrect credential scope for %s", field.name)
					}

					if !field.required && slices.ContainsFunc(env, func(value string) bool { return strings.HasPrefix(value, field.name+"=") }) {
						t.Fatal("an inherited credential reached a tool that does not need it")
					}
				}

				return nil
			}

			if err := runtime.launch(tc.command, tc.args); err != nil || !called {
				t.Fatal("tool was not launched")
			}
		})
	}

	for _, args := range [][]string{{"ops"}, {"ops", "status"}, {"key"}, {"key", "../other"}, {"init", "unexpected"}, {"unknown"}} {
		if err := runtime.run(args, strings.NewReader(""), io.Discard); err == nil {
			t.Fatal("invalid command was accepted")
		}
	}
}

func TestStatusAuthenticatesAndDoesNotExposeKeys(t *testing.T) {
	runtime := newTestRuntime(t)
	initialiseForTest(t, runtime, "init", "")
	api, management := keyForTest(t, runtime, apiField), keyForTest(t, runtime, managementField)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz", "/management.html":
			if r.Header.Get("Authorization") != "" {
				t.Error("public health endpoint received a credential")
			}
		case "/v1/models":
			if r.Header.Get("Authorization") != "Bearer "+api {
				t.Error("wrong client key")
			}

			io.WriteString(w, `{"data":[{"id":"model"}]}`)
		case "/v0/management/auth-files":
			if r.Header.Get("Authorization") != "Bearer "+management {
				t.Error("wrong management key")
			}

			io.WriteString(w, `{"files":[{},{}]}`)
		default:
			http.Error(w, management, http.StatusUnauthorized)
		}
	}))
	t.Cleanup(server.Close)
	runtime.url, runtime.client = server.URL, server.Client()

	var output bytes.Buffer

	if err := runtime.run(nil, strings.NewReader(""), &output); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output.String(), "1 models") || !strings.Contains(output.String(), "2 provider accounts") || strings.Contains(output.String(), api) || strings.Contains(output.String(), management) {
		t.Fatal("status output is incorrect or exposed a credential")
	}

	_, err := runtime.request("/failure", managementField)

	if err == nil || !strings.Contains(err.Error(), "HTTP 401") || strings.Contains(err.Error(), management) {
		t.Fatal("HTTP failure leaked its response or hid its status")
	}
}

func TestRequestsDoNotForwardCredentialsOnRedirect(t *testing.T) {
	runtime := newTestRuntime(t)
	initialiseForTest(t, runtime, "init", "")
	forwarded := false
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { forwarded = true }))
	t.Cleanup(target.Close)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	t.Cleanup(server.Close)
	runtime.url = server.URL
	runtime.client = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	if _, err := runtime.request("/v1/models", apiField); err == nil || forwarded {
		t.Fatal("authenticated redirect was followed")
	}
}

func TestReadKeyAndExecutionErrorsFailClosed(t *testing.T) {
	runtime := newTestRuntime(t)

	if err := runtime.launch("quota", nil); err == nil {
		t.Fatal("tool launched without its credential")
	}

	initialiseForTest(t, runtime, "init", "")
	failure := errors.New("cannot execute tool")
	runtime.execute = func(string, []string, []string) error { return failure }

	if err := runtime.launch("quota", nil); !errors.Is(err, failure) {
		t.Fatal("execution failure was ignored")
	}

	var output bytes.Buffer

	if err := runtime.run([]string{"key", apiField}, strings.NewReader(""), &output); err != nil || strings.TrimSuffix(output.String(), "\n") != keyForTest(t, runtime, apiField) {
		t.Fatal("explicit key command did not return its selected credential")
	}
}
