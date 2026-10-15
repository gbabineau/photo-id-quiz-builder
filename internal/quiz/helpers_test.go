package quiz

import (
	"os"
	"strings"
)

func writeRaw(path, content string) error { return os.WriteFile(path, []byte(content), 0o644) }

func contains(s, sub string) bool { return strings.Contains(s, sub) }
