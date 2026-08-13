package debug

import (
	"io"
	"os"
	"testing"
)

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestLog_WritesWhenEnabled(t *testing.T) {
	orig := Enabled
	Enabled = true
	defer func() { Enabled = orig }()

	out := captureStderr(t, func() { Log("hello %s", "world") })
	if out != "[debug] hello world\n" {
		t.Errorf("output = %q", out)
	}
}

func TestLog_NoopWhenDisabled(t *testing.T) {
	orig := Enabled
	Enabled = false
	defer func() { Enabled = orig }()

	out := captureStderr(t, func() { Log("hello %s", "world") })
	if out != "" {
		t.Errorf("expected no output, got %q", out)
	}
}
