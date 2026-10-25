package flaky

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func installerFixture(t *testing.T, platform string) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "tools")
	if err := os.Mkdir(bin, 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"curl": `#!/bin/sh
output=
url=
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o) output=$2; shift 2 ;;
    -*) shift ;;
    *) url=$1; shift ;;
  esac
done
case "$url" in
  https://api.github.com/repos/dpologdvinity/flaky/releases/latest)
    printf '{"tag_name":"v0.1.1"}\n' ;;
  https://github.com/dpologdvinity/flaky/releases/download/v0.1.1/checksums.txt)
    cp "$FLAKY_TEST_FIXTURES/checksums.txt" "$output" ;;
  https://github.com/dpologdvinity/flaky/releases/download/v0.1.1/*.tar.gz)
    [ "${FLAKY_TEST_DOWNLOAD_FAIL:-0}" = 0 ] || exit 22
    cp "$FLAKY_TEST_FIXTURES/${url##*/}" "$output" ;;
  *) echo "unexpected URL: $url" >&2; exit 22 ;;
esac
`,
		"uname": "#!/bin/sh\ncase \"$1\" in -s) echo \"$FLAKY_TEST_OS\";; -m) echo \"$FLAKY_TEST_ARCH\";; *) exit 1;; esac\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0755); err != nil {
			t.Fatal(err)
		}
	}
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	body := []byte("#!/bin/sh\nprintf 'flaky 0.1.1\\n'\n")
	if err := tw.WriteHeader(&tar.Header{Name: "flaky", Mode: 0755, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	name := "flaky_0.1.1_" + platform + ".tar.gz"
	if err := os.WriteFile(filepath.Join(dir, name), archive.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	sums := fmt.Sprintf("%x  %s\n", sha256.Sum256(archive.Bytes()), name)
	if err := os.WriteFile(filepath.Join(dir, "checksums.txt"), []byte(sums), 0644); err != nil {
		t.Fatal(err)
	}
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate installer")
	}
	return dir, bin, filepath.Join(filepath.Dir(source), "..", "..", "install.sh")
}

func runInstaller(t *testing.T, dir, tools, script, system, arch string, extra ...string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command("sh", script)
	cmd.Env = append(os.Environ(), "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FLAKY_TEST_FIXTURES="+dir, "FLAKY_TEST_OS="+system, "FLAKY_TEST_ARCH="+arch,
		"FLAKY_INSTALL_DIR="+filepath.Join(dir, "install dir"))
	cmd.Env = append(cmd.Env, extra...)
	return cmd.CombinedOutput()
}

func TestInstallerPlatforms(t *testing.T) {
	for _, tc := range []struct{ system, arch, platform string }{
		{"Linux", "x86_64", "linux_amd64"}, {"Linux", "aarch64", "linux_arm64"},
		{"Darwin", "x86_64", "darwin_amd64"}, {"Darwin", "arm64", "darwin_arm64"},
	} {
		t.Run(tc.platform, func(t *testing.T) {
			dir, tools, script := installerFixture(t, tc.platform)
			output, err := runInstaller(t, dir, tools, script, tc.system, tc.arch)
			if err != nil {
				t.Fatalf("%s: %v", output, err)
			}
			binary := filepath.Join(dir, "install dir", "flaky")
			info, err := os.Stat(binary)
			if err != nil || info.Mode().Perm() != 0755 {
				t.Fatalf("installed permissions: %v %v", info, err)
			}
			output, err = exec.Command(binary, "--version").CombinedOutput()
			if err != nil || string(output) != "flaky 0.1.1\n" {
				t.Fatalf("%s: %v", output, err)
			}
		})
	}
}

func TestInstallerFailurePreservesExistingBinary(t *testing.T) {
	for _, problem := range []string{"checksum", "download", "unsupported OS", "unsupported architecture"} {
		t.Run(problem, func(t *testing.T) {
			dir, tools, script := installerFixture(t, "linux_amd64")
			installDir := filepath.Join(dir, "install dir")
			if err := os.Mkdir(installDir, 0755); err != nil {
				t.Fatal(err)
			}
			binary := filepath.Join(installDir, "flaky")
			if err := os.WriteFile(binary, []byte("existing binary"), 0755); err != nil {
				t.Fatal(err)
			}
			system, arch := "Linux", "x86_64"
			var extra []string
			switch problem {
			case "checksum":
				if err := os.WriteFile(filepath.Join(dir, "checksums.txt"), []byte(strings.Repeat("0", 64)+"  flaky_0.1.1_linux_amd64.tar.gz\n"), 0644); err != nil {
					t.Fatal(err)
				}
			case "download":
				extra = append(extra, "FLAKY_TEST_DOWNLOAD_FAIL=1")
			case "unsupported OS":
				system = "Plan9"
			case "unsupported architecture":
				arch = "riscv64"
			}
			if output, err := runInstaller(t, dir, tools, script, system, arch, extra...); err == nil {
				t.Fatalf("accepted %s: %s", problem, output)
			}
			data, err := os.ReadFile(binary)
			if err != nil || string(data) != "existing binary" {
				t.Fatalf("existing binary changed: %q %v", data, err)
			}
		})
	}
}
